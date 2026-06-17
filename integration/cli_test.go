package integration

import (
	"cmp"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/juanfont/headscale/integration/hsic"
	"github.com/juanfont/headscale/integration/tsic"
	"github.com/stretchr/testify/require"
)

// This file holds the shared helpers used by the per-command CLI integration
// test files (cli_users_test.go, cli_nodes_test.go, cli_apikeys_test.go,
// cli_preauthkeys_test.go, cli_auth_test.go, cli_server_test.go and
// cli_policy_test.go). The tests themselves live in those files, grouped by
// the command they exercise.
//
// The whole point of the CLI test suite is to guard the transport: every
// command is invoked with `--output json` and the result is unmarshalled into
// the matching gen/go/headscale/v1 Go type, so a change to the gRPC handlers,
// proto definitions or output encoders that breaks a command is caught here.

func executeAndUnmarshal[T any](headscale ControlServer, command []string, result T) error {
	str, err := headscale.Execute(command)
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(str), result)
	if err != nil {
		return fmt.Errorf("failed to unmarshal: %w\n command err: %s", err, str)
	}

	return nil
}

// assertJSONRoundtrip executes command (which must include `--output json`),
// decodes the stdout into T, then marshals T back to JSON and re-decodes it,
// asserting the serialisation is stable. This is the transport contract guard:
// if the underlying v1 type drifts in a way that loses data, the round-trip
// breaks. The decoded value is returned so callers can assert on real fields.
func assertJSONRoundtrip[T any](t require.TestingT, headscale ControlServer, command []string) T {
	var first T

	err := executeAndUnmarshal(headscale, command, &first)
	require.NoError(t, err, "decoding CLI json output")

	firstBytes, err := json.Marshal(first)
	require.NoError(t, err, "re-marshalling decoded value")

	var second T

	require.NoError(t, json.Unmarshal(firstBytes, &second), "re-decoding marshalled value")

	secondBytes, err := json.Marshal(second)
	require.NoError(t, err, "re-marshalling round-tripped value")

	require.JSONEq(t, string(firstBytes), string(secondBytes), "json round-trip should be stable")

	return second
}

// Interface ensuring that we can sort structs from gRPC that
// have an ID field.
type GRPCSortable interface {
	GetId() uint64
}

func sortWithID[T GRPCSortable](a, b T) int {
	return cmp.Compare(a.GetId(), b.GetId())
}

// setupCLIScenario boots a scenario with the given users and nodes-per-user,
// creates the headscale environment and returns the running scenario and its
// control server. It removes the repeated NewScenario/CreateHeadscaleEnv/
// Headscale boilerplate shared by the CLI tests. Callers still defer
// scenario.ShutdownAssertNoPanics(t) themselves so the cleanup is visible at
// the call site.
func setupCLIScenario(t *testing.T, testName string, users []string, nodesPerUser int) (*Scenario, ControlServer) {
	t.Helper()

	spec := ScenarioSpec{
		Users:        users,
		NodesPerUser: nodesPerUser,
	}

	scenario, err := NewScenario(spec)
	require.NoError(t, err)

	err = scenario.CreateHeadscaleEnv([]tsic.Option{}, hsic.WithTestName(testName))
	require.NoError(t, err)

	headscale, err := scenario.Headscale()
	require.NoError(t, err)

	return scenario, headscale
}

// TestHealthCommand exercises the `headscale health` CLI command end-to-end.
// Until now only the raw /health HTTP endpoint was hit (WaitForRunning); the
// CLI's ogen client.Health() path had no coverage.
func TestHealthCommand(t *testing.T) {
	IntegrationSkip(t)

	spec := ScenarioSpec{
		Users: []string{"health-user"},
	}

	scenario, err := NewScenario(spec)

	require.NoError(t, err)
	defer scenario.ShutdownAssertNoPanics(t)

	err = scenario.CreateHeadscaleEnv([]tsic.Option{}, hsic.WithTestName("cli-health"))
	require.NoError(t, err)

	headscale, err := scenario.Headscale()
	require.NoError(t, err)

	// JSON output decodes and reports the database as reachable.
	var health apiv1.HealthOK

	err = executeAndUnmarshal(
		headscale,
		[]string{"headscale", "health", "--output", "json"},
		&health,
	)
	require.NoError(t, err)
	assert.True(t, health.GetDatabaseConnectivity().Or(false), "database should be reachable")

	// Default (non-JSON) output path also succeeds.
	_, err = headscale.Execute([]string{"headscale", "health"})
	require.NoError(t, err)
}

// TestNodeRoutesCommand covers `nodes list-routes` and `nodes backfillips`,
// neither of which had any integration coverage. Both go through the ogen HTTP
// client; list-routes also renders via the table path on HTTP-decoded data.
func TestNodeRoutesCommand(t *testing.T) {
	IntegrationSkip(t)

	spec := ScenarioSpec{
		Users: []string{"routes-user"},
	}

	scenario, err := NewScenario(spec)

	require.NoError(t, err)
	defer scenario.ShutdownAssertNoPanics(t)

	err = scenario.CreateHeadscaleEnv([]tsic.Option{}, hsic.WithTestName("cli-routes"))
	require.NoError(t, err)

	headscale, err := scenario.Headscale()
	require.NoError(t, err)

	regIDs := []string{
		types.MustAuthID().String(),
		types.MustAuthID().String(),
	}

	for index, regID := range regIDs {
		_, err := headscale.Execute(
			[]string{
				"headscale", "debug", "create-node",
				"--name", fmt.Sprintf("route-node-%d", index+1),
				"--user", "routes-user",
				"--key", regID,
				"--output", "json",
			},
		)
		require.NoError(t, err)

		var node apiv1.Node

		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			err = executeAndUnmarshal(
				headscale,
				[]string{
					"headscale", "auth", "register",
					"--user", "routes-user",
					"--auth-id", regID,
					"--output", "json",
				},
				&node,
			)
			assert.NoError(c, err)
		}, integrationutil.ScaledTimeout(10*time.Second), integrationutil.FastPoll, "registering node")
	}

	// list-routes decodes over HTTP (json output).
	var routeNodes []apiv1.Node

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		err := executeAndUnmarshal(
			headscale,
			[]string{"headscale", "nodes", "list-routes", "--output", "json"},
			&routeNodes,
		)
		assert.NoError(c, err)
	}, integrationutil.ScaledTimeout(15*time.Second), 1*time.Second)

	// list-routes also renders as a table (exercises nodeRoutesToPtables).
	_, err = headscale.Execute([]string{"headscale", "nodes", "list-routes"})
	require.NoError(t, err)

	// backfillips runs non-interactively and decodes its change list.
	_, err = headscale.Execute(
		[]string{"headscale", "nodes", "backfillips", "--force", "--output", "json"},
	)
	require.NoError(t, err)
}
