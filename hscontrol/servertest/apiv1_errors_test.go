package servertest_test

import (
	"context"
	"net/http"
	"testing"

	apiv1 "github.com/juanfont/headscale/gen/api/v1"
)

// The not-found cases below exercise the gRPC->HTTP error translation: a state
// "record not found" must surface as an RFC 7807 404, not a 500 or a success.

func TestAPIv1_Nodes_NotFound(t *testing.T) {
	_, client := apiClient(t)
	ctx := context.Background()

	const missing = uint64(99999)

	requireProblem(t, client.DeleteNode(ctx, apiv1.DeleteNodeParams{NodeID: missing}), http.StatusNotFound)

	_, err := client.RenameNode(ctx, apiv1.RenameNodeParams{NodeID: missing, NewName: "x"})
	requireProblem(t, err, http.StatusNotFound)

	_, err = client.ExpireNode(ctx, apiv1.ExpireNodeParams{NodeID: missing})
	requireProblem(t, err, http.StatusNotFound)
}

func TestAPIv1_Users_NotFound(t *testing.T) {
	_, client := apiClient(t)
	ctx := context.Background()

	const missing = uint64(99999)

	requireProblem(t, client.DeleteUser(ctx, apiv1.DeleteUserParams{ID: missing}), http.StatusNotFound)

	_, err := client.RenameUser(ctx, apiv1.RenameUserParams{OldID: missing, NewName: "x"})
	requireProblem(t, err, http.StatusNotFound)
}

func TestAPIv1_ApiKeys_NotFound(t *testing.T) {
	_, client := apiClient(t)
	ctx := context.Background()

	requireProblem(t, client.ExpireApiKey(ctx, &apiv1.ExpireApiKeyReq{
		ID: apiv1.NewOptUint64(99999),
	}), http.StatusNotFound)

	requireProblem(t, client.DeleteApiKey(ctx, apiv1.DeleteApiKeyParams{
		Prefix: "nonexistent",
	}), http.StatusNotFound)
}

func TestAPIv1_PreAuthKeys_Errors(t *testing.T) {
	_, client := apiClient(t)
	ctx := context.Background()

	requireProblem(t, client.ExpirePreAuthKey(ctx, &apiv1.ExpirePreAuthKeyReq{
		ID: apiv1.NewOptUint64(99999),
	}), http.StatusNotFound)

	requireProblem(t, client.DeletePreAuthKey(ctx, apiv1.DeletePreAuthKeyParams{
		ID: apiv1.NewOptUint64(99999),
	}), http.StatusNotFound)

	// Creating a key for a user that does not exist is a 404.
	_, err := client.CreatePreAuthKey(ctx, &apiv1.CreatePreAuthKeyReq{
		User: apiv1.NewOptUint64(99999),
	})
	requireProblem(t, err, http.StatusNotFound)
}

func TestAPIv1_SetApprovedRoutes_InvalidCIDR(t *testing.T) {
	srv, client := apiClient(t)
	ctx := context.Background()

	user := srv.CreateUser(t, "route-user")
	node := srv.CreateNode(t, user, "route-node")

	_, err := client.SetApprovedRoutes(ctx,
		&apiv1.SetApprovedRoutesReq{Routes: []string{"not-a-cidr"}},
		apiv1.SetApprovedRoutesParams{NodeID: uint64(node.ID)},
	)
	requireProblem(t, err, http.StatusBadRequest)
}

func TestAPIv1_SetPolicy_Invalid(t *testing.T) {
	_, client := apiClient(t)
	ctx := context.Background()

	_, err := client.SetPolicy(ctx, &apiv1.SetPolicyReq{
		Policy: apiv1.NewOptString("{ this is not valid hujson"),
	})
	requireProblem(t, err, http.StatusBadRequest)
}
