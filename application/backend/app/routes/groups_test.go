package routes

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/devforth/OnLogs/app/groups"
	"github.com/devforth/OnLogs/app/hostalias"
)

func groupNames(t *testing.T, body []byte) []string {
	t.Helper()

	var list []groups.Group
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshalling the group list: %v -- body: %s", err, body)
	}
	names := []string{}
	for _, group := range list {
		names = append(names, group.Name)
	}
	return names
}

func TestGroupsAreIsolatedPerUser(t *testing.T) {
	t.Cleanup(func() { groups.Delete("testuser", "isolated") })

	status, body := call(t, testCtrl.CreateGroup, authedRequest(t, "testuser", "POST", "/", map[string]interface{}{
		"name":    "isolated",
		"members": []map[string]string{{"host": "Test1", "service": "containerTest1"}},
	}))
	if status != http.StatusOK {
		t.Fatalf("creating the group returned %d: %s", status, body)
	}

	status, body = call(t, testCtrl.GetGroups, authedRequest(t, "viewer", "GET", "/", nil))
	if status != http.StatusOK {
		t.Fatalf("viewer's getGroups returned %d: %s", status, body)
	}
	if slices.Contains(groupNames(t, body), "isolated") {
		t.Fatalf("viewer can read testuser's group: %s", body)
	}

	call(t, testCtrl.DeleteGroup, authedRequest(t, "viewer", "POST", "/", map[string]string{"name": "isolated"}))
	if _, found, _ := groups.Load("testuser", "isolated"); !found {
		t.Fatal("viewer's deleteGroup removed testuser's group")
	}

	call(t, testCtrl.UpdateGroup, authedRequest(t, "viewer", "POST", "/", map[string]interface{}{
		"name":    "isolated",
		"members": []map[string]string{},
	}))
	members, found, _ := groups.Load("testuser", "isolated")
	if !found || len(members) != 1 {
		t.Fatalf("viewer's updateGroup changed testuser's group: found=%v members=%v", found, members)
	}
}

func TestGroupCRUDRoundTrip(t *testing.T) {
	t.Cleanup(func() {
		groups.Delete("testuser", "backend")
		groups.Delete("testuser", "backend renamed")
	})

	status, body := call(t, testCtrl.CreateGroup, authedRequest(t, "testuser", "POST", "/", map[string]interface{}{
		"name": "backend",
		"members": []map[string]string{
			{"host": "Test1", "service": "containerTest1"},
			{"host": "Test2", "service": "containerTest2"},
		},
	}))
	if status != http.StatusOK {
		t.Fatalf("createGroup returned %d: %s", status, body)
	}

	status, body = call(t, testCtrl.GetGroups, authedRequest(t, "testuser", "GET", "/", nil))
	if status != http.StatusOK || !slices.Contains(groupNames(t, body), "backend") {
		t.Fatalf("getGroups returned %d without the new group: %s", status, body)
	}

	status, body = call(t, testCtrl.UpdateGroup, authedRequest(t, "testuser", "POST", "/", map[string]interface{}{
		"name":    "backend",
		"newName": "backend renamed",
		"members": []map[string]string{{"host": "Test1", "service": "containerTest1"}},
	}))
	if status != http.StatusOK {
		t.Fatalf("updateGroup returned %d: %s", status, body)
	}

	if _, found, _ := groups.Load("testuser", "backend"); found {
		t.Fatal("the old name still resolves after a rename")
	}
	members, found, _ := groups.Load("testuser", "backend renamed")
	if !found || len(members) != 1 || members[0].Service != "containerTest1" {
		t.Fatalf("after the update the group holds %v", members)
	}

	status, body = call(t, testCtrl.DeleteGroup, authedRequest(t, "testuser", "POST", "/", map[string]string{
		"name": "backend renamed",
	}))
	if status != http.StatusOK {
		t.Fatalf("deleteGroup returned %d: %s", status, body)
	}
	if _, found, _ := groups.Load("testuser", "backend renamed"); found {
		t.Fatal("the group survived deleteGroup")
	}
}

func TestGroupRoutesRejectUnauthenticated(t *testing.T) {

	for _, route := range []struct {
		name    string
		handler http.HandlerFunc
		method  string
		body    interface{}
	}{
		{"getGroups", testCtrl.GetGroups, "GET", nil},
		{"createGroup", testCtrl.CreateGroup, "POST", map[string]interface{}{"name": "nope", "members": []string{}}},
		{"updateGroup", testCtrl.UpdateGroup, "POST", map[string]interface{}{"name": "nope", "members": []string{}}},
		{"deleteGroup", testCtrl.DeleteGroup, "POST", map[string]string{"name": "nope"}},
	} {
		t.Run(route.name, func(t *testing.T) {
			status, body := call(t, route.handler, authedRequest(t, "", route.method, "/", route.body))
			if status != http.StatusUnauthorized {
				t.Fatalf("%s returned %d for an unauthenticated request: %s", route.name, status, body)
			}
		})
	}

	if _, found, _ := groups.Load("", "nope"); found {
		groups.Delete("", "nope")
		t.Fatal("an unauthenticated request wrote a group")
	}
}

func TestCreateGroupRejectsDuplicateName(t *testing.T) {
	t.Cleanup(func() { groups.Delete("testuser", "duplicated") })

	create := func() (int, []byte) {
		return call(t, testCtrl.CreateGroup, authedRequest(t, "testuser", "POST", "/", map[string]interface{}{
			"name":    "duplicated",
			"members": []map[string]string{{"host": "Test1", "service": "containerTest1"}},
		}))
	}

	if status, body := create(); status != http.StatusOK {
		t.Fatalf("the first createGroup returned %d: %s", status, body)
	}
	if status, body := create(); status != http.StatusConflict {
		t.Fatalf("the second createGroup returned %d, want 409: %s", status, body)
	}
}

func TestUpdateGroupOnMissingGroupIsNotFound(t *testing.T) {

	status, body := call(t, testCtrl.UpdateGroup, authedRequest(t, "testuser", "POST", "/", map[string]interface{}{
		"name":    "never created",
		"members": []map[string]string{{"host": "Test1", "service": "containerTest1"}},
	}))
	if status != http.StatusNotFound {
		t.Fatalf("updateGroup on a missing group returned %d, want 404: %s", status, body)
	}
	if _, found, _ := groups.Load("testuser", "never created"); found {
		groups.Delete("testuser", "never created")
		t.Fatal("updateGroup silently created the group")
	}
}

// The UI can double-fire delete, so the second one is not an error.
func TestDeleteGroupIsIdempotent(t *testing.T) {

	for attempt := 0; attempt < 2; attempt++ {
		status, body := call(t, testCtrl.DeleteGroup, authedRequest(t, "testuser", "POST", "/", map[string]string{
			"name": "was never there",
		}))
		if status != http.StatusOK {
			t.Fatalf("deleteGroup attempt %d returned %d, want 200: %s", attempt+1, status, body)
		}
	}
}

func TestGroupRoutesRejectMalformedInput(t *testing.T) {

	cases := []struct {
		label   string
		handler http.HandlerFunc
		body    interface{}
	}{
		{"name carrying the key separator", testCtrl.CreateGroup, map[string]interface{}{
			"name": "web\x00admin", "members": []map[string]string{},
		}},
		{"empty name", testCtrl.CreateGroup, map[string]interface{}{
			"name": "", "members": []map[string]string{},
		}},
		{"oversized name", testCtrl.CreateGroup, map[string]interface{}{
			"name":    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"members": []map[string]string{},
		}},
		{"member escaping its directory", testCtrl.CreateGroup, map[string]interface{}{
			"name": "traversal", "members": []map[string]string{{"host": "..", "service": "api"}},
		}},
		{"member with a path separator", testCtrl.CreateGroup, map[string]interface{}{
			"name": "separator", "members": []map[string]string{{"host": "Test1", "service": "a/b"}},
		}},
		{"member with an empty service", testCtrl.CreateGroup, map[string]interface{}{
			"name": "empty member", "members": []map[string]string{{"host": "Test1", "service": ""}},
		}},
		{"delete with a forged name", testCtrl.DeleteGroup, map[string]string{"name": "web\x00admin"}},
		{"update with a forged new name", testCtrl.UpdateGroup, map[string]interface{}{
			"name": "backend", "newName": "web\x00admin", "members": []map[string]string{},
		}},
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			status, body := call(t, c.handler, authedRequest(t, "testuser", "POST", "/", c.body))
			if status != http.StatusBadRequest {
				t.Fatalf("returned %d, want 400: %s", status, body)
			}
		})
	}
}

func TestCreateGroupCapsGroupsPerUser(t *testing.T) {

	names := []string{}
	t.Cleanup(func() {
		for _, name := range names {
			groups.Delete("someuser", name)
		}
	})

	for i := 0; i < groups.MaxGroupsPerUser; i++ {
		name := "capped-" + string(rune('a'+i/26)) + string(rune('a'+i%26))
		names = append(names, name)
		if err := groups.Store("someuser", name, []groups.Member{}); err != nil {
			t.Fatalf("seeding group %d: %v", i, err)
		}
	}

	names = append(names, "one too many")
	status, body := call(t, testCtrl.CreateGroup, authedRequest(t, "someuser", "POST", "/", map[string]interface{}{
		"name":    "one too many",
		"members": []map[string]string{},
	}))
	if status != http.StatusBadRequest {
		t.Fatalf("createGroup past the cap returned %d, want 400: %s", status, body)
	}
	if _, found, _ := groups.Load("someuser", "one too many"); found {
		t.Fatal("createGroup stored a group past the cap")
	}
}

// DISABLE_AUTH is accepted as-is, not fixed here: util.GetUserFromJWT still
// returns "" for a cookie-less request, so every such user shares one bucket.
// Favourites already behave this way. This pins it as a decision.
func TestDisableAuthSharesOneGroupBucket(t *testing.T) {
	t.Setenv("DISABLE_AUTH", "true")
	t.Cleanup(func() { groups.Delete("", "shared bucket") })

	status, body := call(t, testCtrl.CreateGroup, authedRequest(t, "", "POST", "/", map[string]interface{}{
		"name":    "shared bucket",
		"members": []map[string]string{{"host": "Test1", "service": "containerTest1"}},
	}))
	if status != http.StatusOK {
		t.Fatalf("createGroup under DISABLE_AUTH returned %d: %s", status, body)
	}

	status, body = call(t, testCtrl.GetGroups, authedRequest(t, "", "GET", "/", nil))
	if status != http.StatusOK || !slices.Contains(groupNames(t, body), "shared bucket") {
		t.Fatalf("a second anonymous session did not see the group: %d %s", status, body)
	}

	status, body = call(t, testCtrl.CreateGroup, authedRequest(t, "", "POST", "/", map[string]interface{}{
		"name":    "shared bucket",
		"members": []map[string]string{},
	}))
	if status != http.StatusConflict {
		t.Fatalf("a second anonymous createGroup returned %d, want 409: %s", status, body)
	}
}

func TestHostAliasIsAdminOnlyToSet(t *testing.T) {
	t.Cleanup(func() { hostalias.Set("aliashost", "") })

	status, body := call(t, testCtrl.SetHostAlias,
		authedRequest(t, "viewer", "POST", "/", map[string]string{"host": "aliashost", "alias": "prod"}))
	if status != http.StatusForbidden {
		t.Fatalf("a non-admin set a host alias: status %d, %s", status, body)
	}
	if aliases, _ := hostalias.All(); aliases["aliashost"] != "" {
		t.Fatalf("a non-admin's alias was stored: %q", aliases["aliashost"])
	}

	status, body = call(t, testCtrl.SetHostAlias,
		authedRequest(t, "admin", "POST", "/", map[string]string{"host": "aliashost", "alias": "prod"}))
	if status != http.StatusOK {
		t.Fatalf("admin could not set a host alias: status %d, %s", status, body)
	}

	status, body = call(t, testCtrl.GetHostAliases, authedRequest(t, "viewer", "GET", "/", nil))
	if status != http.StatusOK {
		t.Fatalf("a normal user could not read host aliases: status %d, %s", status, body)
	}
	var aliases map[string]string
	if err := json.Unmarshal(body, &aliases); err != nil {
		t.Fatalf("unmarshalling aliases: %v -- %s", err, body)
	}
	if aliases["aliashost"] != "prod" {
		t.Errorf("getHostAliases returned %v, want aliashost=prod", aliases)
	}
}

func TestHostAliasRejectsAnUnsafeHost(t *testing.T) {
	status, _ := call(t, testCtrl.SetHostAlias,
		authedRequest(t, "admin", "POST", "/", map[string]string{"host": "../../etc", "alias": "pwn"}))
	if status != http.StatusBadRequest {
		t.Fatalf("a traversing host was accepted: status %d", status)
	}
}
