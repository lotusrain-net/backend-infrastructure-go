package iam

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"backend-infrastructure-go/internal/shared/pagination"
)

type memoryRefreshCache struct {
	mu     sync.Mutex
	values map[string]string
}

type recordingPasswords struct {
	verifyCalls int
	lastHash    string
}

type failingEvalCache struct{ err error }

func (f failingEvalCache) Get(context.Context, string) (string, error) { return "", f.err }
func (f failingEvalCache) Eval(context.Context, string, []string, ...any) (any, error) {
	return nil, f.err
}

type failingUsers struct{ err error }

func (f failingUsers) FindByEmail(context.Context, string) (User, error) { return User{}, f.err }
func (f failingUsers) FindByID(context.Context, string) (User, error)    { return User{}, f.err }
func (f failingUsers) List(context.Context, UserFilter, int, int) ([]User, int64, error) {
	return nil, 0, f.err
}
func (f failingUsers) Create(context.Context, CreateUserInput) (User, error) {
	return User{}, f.err
}
func (f failingUsers) SetActive(context.Context, string, bool) error { return f.err }

type rotateFailureCache struct{ err error }

func (f rotateFailureCache) Get(context.Context, string) (string, error) { return "user-1", nil }
func (f rotateFailureCache) Eval(context.Context, string, []string, ...any) (any, error) {
	return nil, f.err
}

func (p *recordingPasswords) Hash(string) (string, error) { return "hash", nil }
func (p *recordingPasswords) Verify(_ string, encoded string) (bool, error) {
	p.verifyCalls++
	p.lastHash = encoded
	return false, nil
}

func (m *memoryRefreshCache) Get(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.values[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}
func (m *memoryRefreshCache) Eval(_ context.Context, script string, keys []string, args ...any) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.values == nil {
		m.values = map[string]string{}
	}
	return evalRefreshScript(m.values, script, keys, args...)
}

type concurrentIssueCache struct {
	mu        sync.Mutex
	values    map[string]string
	userReads int
	read      chan struct{}
}

func newConcurrentIssueCache() *concurrentIssueCache {
	return &concurrentIssueCache{values: map[string]string{}, read: make(chan struct{})}
}

func (c *concurrentIssueCache) Get(_ context.Context, key string) (string, error) {
	c.mu.Lock()
	value, ok := c.values[key]
	if strings.Contains(key, ":user:") {
		c.userReads++
		if c.userReads == 2 {
			close(c.read)
		}
		c.mu.Unlock()
		<-c.read
	} else {
		c.mu.Unlock()
	}
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

func (c *concurrentIssueCache) Eval(_ context.Context, script string, keys []string, args ...any) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return evalRefreshScript(c.values, script, keys, args...)
}

func evalRefreshScript(values map[string]string, script string, keys []string, args ...any) (any, error) {
	stringArg := func(index int) string { return args[index].(string) }
	switch script {
	case replaceRefreshScript:
		if previous := values[keys[1]]; previous != "" {
			delete(values, stringArg(0)+previous)
		}
		values[keys[0]] = stringArg(1)
		values[keys[1]] = stringArg(2)
		return int64(1), nil
	case rotateRefreshScript:
		if values[keys[0]] != stringArg(0) || values[keys[2]] != stringArg(1) {
			return int64(0), nil
		}
		delete(values, keys[0])
		values[keys[1]] = stringArg(0)
		values[keys[2]] = stringArg(2)
		return int64(1), nil
	case consumeRefreshScript:
		userID := values[keys[0]]
		if userID == "" || values[keys[1]] != stringArg(0) {
			return nil, nil
		}
		delete(values, keys[0])
		delete(values, keys[1])
		return userID, nil
	case revokeRefreshScript:
		if values[keys[0]] == "" {
			return int64(0), nil
		}
		delete(values, keys[0])
		if values[keys[1]] == stringArg(0) {
			delete(values, keys[1])
		}
		return int64(1), nil
	case revokeAllRefreshScript:
		hash := values[keys[0]]
		if hash == "" {
			return int64(0), nil
		}
		delete(values, stringArg(0)+hash)
		delete(values, keys[0])
		return int64(1), nil
	default:
		return nil, errors.New("unknown refresh script")
	}
}

func TestRefreshTokensStoredHashedConsumedAndCannotReplay(t *testing.T) {
	cache := &memoryRefreshCache{}
	store := NewRefreshStore(cache, "iam:test", time.Hour)
	raw, err := store.Issue(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(mapKeys(cache.values), ""), raw) {
		t.Fatal("raw refresh token used as cache key")
	}
	if strings.Contains(strings.Join(mapValues(cache.values), ""), raw) {
		t.Fatal("raw refresh token stored as cache value")
	}
	userID, err := store.Consume(context.Background(), raw)
	if err != nil || userID != "user-1" {
		t.Fatalf("user=%q err=%v", userID, err)
	}
	if _, err := store.Consume(context.Background(), raw); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("replay error=%v", err)
	}
}

func TestRefreshRevokeInvalidatesToken(t *testing.T) {
	cache := &memoryRefreshCache{}
	store := NewRefreshStore(cache, "iam:test", time.Hour)
	raw, _ := store.Issue(context.Background(), "user-1")
	if err := store.Revoke(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Consume(context.Background(), raw); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("revoked error=%v", err)
	}
}

func TestConcurrentRefreshIssueLeavesOnlyOneUsableToken(t *testing.T) {
	cache := newConcurrentIssueCache()
	store := NewRefreshStore(cache, "iam:test", time.Hour)
	tokens := make(chan string, 2)
	errors := make(chan error, 2)
	for range 2 {
		go func() {
			token, err := store.Issue(context.Background(), "user-1")
			tokens <- token
			errors <- err
		}()
	}
	issued := []string{<-tokens, <-tokens}
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	usable := 0
	for _, token := range issued {
		if _, err := store.Consume(context.Background(), token); err == nil {
			usable++
		}
	}
	if usable != 1 {
		t.Fatalf("usable concurrent refresh tokens = %d, want 1", usable)
	}
}

type blockingRefreshUsers struct {
	user    User
	entered chan struct{}
	release chan struct{}
}

func (u *blockingRefreshUsers) FindByEmail(context.Context, string) (User, error) {
	return u.user, nil
}
func (u *blockingRefreshUsers) FindByID(context.Context, string) (User, error) {
	close(u.entered)
	<-u.release
	return u.user, nil
}
func (u *blockingRefreshUsers) List(context.Context, UserFilter, int, int) ([]User, int64, error) {
	return []User{u.user}, 1, nil
}
func (u *blockingRefreshUsers) Create(context.Context, CreateUserInput) (User, error) {
	return User{}, nil
}
func (u *blockingRefreshUsers) SetActive(context.Context, string, bool) error { return nil }

func TestRefreshRotationCannotSurviveConcurrentRevokeAll(t *testing.T) {
	cache := &memoryRefreshCache{}
	store := NewRefreshStore(cache, "iam:test", time.Hour)
	raw, err := store.Issue(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	users := &blockingRefreshUsers{
		user:    User{ID: "user-1", Active: true},
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	jwt, _ := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	service := NewService(users, nil, NewPasswordHasher(DefaultArgon2Params()), jwt, store)
	result := make(chan error, 1)
	go func() {
		_, err := service.Refresh(context.Background(), raw)
		result <- err
	}()
	<-users.entered
	if err := store.RevokeAll(context.Background(), "user-1"); err != nil {
		t.Fatal(err)
	}
	close(users.release)
	if err := <-result; !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Refresh() error = %v, want invalid refresh token after revoke-all", err)
	}
}

type fakeUsers struct {
	byEmail        map[string]User
	byID           map[string]User
	listed         []User
	total          int64
	createErr      error
	setActiveCalls int
}

type fakeRBAC struct {
	permissions []string
}

type fakePreferences struct {
	preferences Preferences
	getErr      error
	putErr      error
	saved       Preferences
	savedUserID string
}

func (f *fakePreferences) GetPreferences(context.Context, string) (Preferences, error) {
	return f.preferences, f.getErr
}

func (f *fakePreferences) PutPreferences(_ context.Context, userID string, preferences Preferences) error {
	f.savedUserID = userID
	f.saved = preferences
	return f.putErr
}

func (f *fakeRBAC) PermissionsForUser(context.Context, string) ([]string, error) {
	return f.permissions, nil
}
func (f *fakeRBAC) Roles(context.Context) ([]Role, error) { return []Role{{Name: "admin"}}, nil }
func (f *fakeRBAC) Permissions(context.Context) ([]Permission, error) {
	return []Permission{{Name: "users:read"}}, nil
}
func (f *fakeRBAC) AssignRole(context.Context, string, string) error      { return nil }
func (f *fakeRBAC) GrantPermission(context.Context, string, string) error { return nil }

func TestPreferencesDefaultOnFirstReadAndValidatedFullReplacement(t *testing.T) {
	users := &fakeUsers{byID: map[string]User{"user-1": {ID: "user-1", Active: true}}}
	preferences := &fakePreferences{getErr: ErrNotFound}
	service := NewServiceWithPreferences(users, nil, preferences, nil, nil, nil)

	got, err := service.Preferences(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("Preferences() error = %v", err)
	}
	if got != DefaultPreferences() {
		t.Fatalf("Preferences() = %+v, want defaults", got)
	}

	accent := "#1A2B3C"
	want := Preferences{Theme: ThemeCyberpunk, ColorMode: ColorModeDark, AccentColor: &accent, FontScale: FontScaleLarge, RadiusScale: RadiusScaleRounded}
	if err := service.PutPreferences(context.Background(), "user-1", want); err != nil {
		t.Fatalf("PutPreferences() error = %v", err)
	}
	if preferences.savedUserID != "user-1" || preferences.saved.Theme != ThemeCyberpunk || preferences.saved.AccentColor == nil || *preferences.saved.AccentColor != accent {
		t.Fatalf("saved preferences = user:%q preferences:%+v", preferences.savedUserID, preferences.saved)
	}

	if err := service.PutPreferences(context.Background(), "user-1", Preferences{}); !errors.Is(err, ErrInvalidUserInput) {
		t.Fatalf("PutPreferences() invalid error = %v", err)
	}
}

func (f *fakeUsers) FindByEmail(_ context.Context, email string) (User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}
func (f *fakeUsers) FindByID(_ context.Context, id string) (User, error) {
	u, ok := f.byID[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}
func (f *fakeUsers) List(_ context.Context, _ UserFilter, _ int, _ int) ([]User, int64, error) {
	if f.byID == nil && f.byEmail == nil && f.listed == nil && f.createErr != nil {
		return nil, 0, f.createErr
	}
	return append([]User(nil), f.listed...), f.total, nil
}
func (f *fakeUsers) Create(_ context.Context, input CreateUserInput) (User, error) {
	if f.createErr != nil {
		return User{}, f.createErr
	}
	return User{ID: "new", Email: input.Email, Username: input.Username, Active: true}, nil
}
func (f *fakeUsers) SetActive(context.Context, string, bool) error {
	f.setActiveCalls++
	return nil
}

func TestLoginRejectsInactiveUserAndBadPassword(t *testing.T) {
	hasher := NewPasswordHasher(DefaultArgon2Params())
	hash, _ := hasher.Hash("secret-password")
	repo := &fakeUsers{byEmail: map[string]User{"inactive@example.com": {ID: "u1", Email: "inactive@example.com", PasswordHash: hash, Active: false}, "active@example.com": {ID: "u2", Email: "active@example.com", PasswordHash: hash, Active: true}}}
	auth := newTestAuth(t, repo)
	if _, err := auth.Login(context.Background(), "inactive@example.com", "secret-password"); !errors.Is(err, ErrInactiveUser) {
		t.Fatalf("inactive error=%v", err)
	}
	if _, err := auth.Login(context.Background(), "active@example.com", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("bad password error=%v", err)
	}
}

func TestLoginVerifiesDummyHashWhenIdentityDoesNotExist(t *testing.T) {
	passwords := &recordingPasswords{}
	service := NewService(&fakeUsers{}, nil, passwords, nil, nil)

	if _, err := service.Login(context.Background(), "missing@example.com", "attempted-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v", err)
	}
	if passwords.verifyCalls != 1 || passwords.lastHash != dummyPasswordHash {
		t.Fatalf("Verify calls = %d, hash = %q", passwords.verifyCalls, passwords.lastHash)
	}
}

func TestLoginPropagatesRepositoryFailure(t *testing.T) {
	want := errors.New("postgres unavailable")
	passwords := &recordingPasswords{}
	service := NewService(failingUsers{err: want}, nil, passwords, nil, nil)

	_, err := service.Login(context.Background(), "user@example.com", "attempted-password")
	if !errors.Is(err, want) {
		t.Fatalf("Login() error = %v, want repository failure", err)
	}
	if passwords.verifyCalls != 1 || passwords.lastHash != dummyPasswordHash {
		t.Fatalf("Verify calls = %d, hash = %q", passwords.verifyCalls, passwords.lastHash)
	}
}

func TestRefreshPropagatesDependencyFailures(t *testing.T) {
	want := errors.New("dependency unavailable")
	jwt, _ := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	t.Run("lookup", func(t *testing.T) {
		service := NewService(&fakeUsers{}, nil, NewPasswordHasher(DefaultArgon2Params()), jwt, NewRefreshStore(failingEvalCache{err: want}, "iam:test", time.Hour))
		if _, err := service.Refresh(context.Background(), "refresh-token"); !errors.Is(err, want) {
			t.Fatalf("Refresh() error = %v, want cache failure", err)
		}
	})
	t.Run("user repository", func(t *testing.T) {
		service := NewService(failingUsers{err: want}, nil, NewPasswordHasher(DefaultArgon2Params()), jwt, NewRefreshStore(rotateFailureCache{}, "iam:test", time.Hour))
		if _, err := service.Refresh(context.Background(), "refresh-token"); !errors.Is(err, want) {
			t.Fatalf("Refresh() error = %v, want repository failure", err)
		}
	})
	t.Run("rotation", func(t *testing.T) {
		users := &fakeUsers{byID: map[string]User{"user-1": {ID: "user-1", Active: true}}}
		service := NewService(users, nil, NewPasswordHasher(DefaultArgon2Params()), jwt, NewRefreshStore(rotateFailureCache{err: want}, "iam:test", time.Hour))
		if _, err := service.Refresh(context.Background(), "refresh-token"); !errors.Is(err, want) {
			t.Fatalf("Refresh() error = %v, want rotation failure", err)
		}
	})
}

func TestCurrentUserPropagatesRepositoryFailure(t *testing.T) {
	want := errors.New("postgres unavailable")
	service := NewService(failingUsers{err: want}, nil, nil, nil, nil)
	if _, err := service.CurrentUser(context.Background(), "user-1"); !errors.Is(err, want) {
		t.Fatalf("CurrentUser() error = %v, want repository failure", err)
	}
}

func TestUsersNormalizesPaginationAndDelegatesFilter(t *testing.T) {
	repo := &fakeUsers{
		listed: []User{{ID: "u1", Email: "alice@example.com", Username: "alice", Active: true}},
		total:  1,
	}
	service := NewService(repo, nil, nil, nil, nil)
	active := true

	page, err := service.Users(context.Background(), UserQuery{
		Filter: UserFilter{Query: "  Alice  ", Active: &active},
		Page:   0,
		Size:   pagination.MaxSize + 10,
	})
	if err != nil {
		t.Fatalf("Users() error = %v", err)
	}
	if len(page.Items) != 1 || page.Meta.Page != 1 || page.Meta.Size != pagination.MaxSize || page.Meta.Total != 1 {
		t.Fatalf("page = %+v", page)
	}
}

func TestCreateUserSurfacesDuplicateIdentity(t *testing.T) {
	auth := newTestAuth(t, &fakeUsers{createErr: ErrDuplicateIdentity})
	_, err := auth.CreateUser(context.Background(), CreateUserInput{Email: "taken@example.com", Username: "taken", Password: "password-long-enough"})
	if !errors.Is(err, ErrDuplicateIdentity) {
		t.Fatalf("error=%v", err)
	}
}

func TestSetUserInactiveFailsWhenRefreshRevocationFails(t *testing.T) {
	want := errors.New("redis unavailable")
	users := &fakeUsers{}
	service := NewService(users, nil, NewPasswordHasher(DefaultArgon2Params()), nil, NewRefreshStore(failingEvalCache{err: want}, "iam:test", time.Hour))

	err := service.SetUserActive(context.Background(), "user-1", false)
	if !errors.Is(err, want) {
		t.Fatalf("SetUserActive() error = %v", err)
	}
	if users.setActiveCalls != 0 {
		t.Fatalf("SetActive() calls = %d, want 0", users.setActiveCalls)
	}
}

func TestServiceLoginRefreshLogoutAndAuthorization(t *testing.T) {
	hasher := NewPasswordHasher(DefaultArgon2Params())
	hash, _ := hasher.Hash("secret-password")
	user := User{ID: "u1", Email: "active@example.com", PasswordHash: hash, Active: true}
	users := &fakeUsers{byEmail: map[string]User{user.Email: user}, byID: map[string]User{user.ID: user}}
	cache := &memoryRefreshCache{}
	jwt, _ := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	rbac := &fakeRBAC{permissions: []string{"users:*"}}
	service := NewService(users, rbac, hasher, jwt, NewRefreshStore(cache, "iam:test", time.Hour))

	pair, err := service.Login(context.Background(), " ACTIVE@EXAMPLE.COM ", "secret-password")
	if err != nil || pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("pair=%+v err=%v", pair, err)
	}
	rotated, err := service.Refresh(context.Background(), pair.RefreshToken)
	if err != nil || rotated.RefreshToken == pair.RefreshToken {
		t.Fatalf("rotated=%+v err=%v", rotated, err)
	}
	if _, err := service.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("replay=%v", err)
	}
	if err := service.Authorize(context.Background(), user.ID, "users:write"); err != nil {
		t.Fatal(err)
	}
	rbac.permissions = []string{"users:read"}
	if err := service.Authorize(context.Background(), user.ID, "users:write"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("deny=%v", err)
	}
	if current, err := service.CurrentUser(context.Background(), user.ID); err != nil || current.ID != user.ID {
		t.Fatalf("current=%+v err=%v", current, err)
	} else if got := strings.Join(current.Permissions, ","); got != "users:read" {
		t.Fatalf("permissions=%q", got)
	}
	if err := service.Logout(context.Background(), rotated.RefreshToken); err != nil {
		t.Fatal(err)
	}
}

func TestRevokeAllInvalidatesLatestRefreshToken(t *testing.T) {
	cache := &memoryRefreshCache{}
	store := NewRefreshStore(cache, "iam:test", time.Hour)
	raw, _ := store.Issue(context.Background(), "u1")
	if err := store.RevokeAll(context.Background(), "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Consume(context.Background(), raw); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("consume=%v", err)
	}
}

type fakeManagement struct {
	updatedInput   UpdateUserInput
	updatedUser    User
	updateErr      error
	passwordHash   string
	passwordErr    error
	activeUserID   string
	active         bool
	activeErr      error
	userRoleIDs    []string
	userRolesErr   error
	role           RoleDetail
	roleErr        error
	createdRole    RoleInput
	updatedRole    RoleInput
	deletedRoleID  string
	permissionIDs  []string
	permissionsErr error
}

func (f *fakeManagement) UpdateUser(_ context.Context, _ string, input UpdateUserInput) (User, error) {
	f.updatedInput = input
	return f.updatedUser, f.updateErr
}
func (f *fakeManagement) UpdateUserPassword(_ context.Context, _ string, passwordHash string) error {
	f.passwordHash = passwordHash
	return f.passwordErr
}
func (f *fakeManagement) SetUserActive(_ context.Context, userID string, active bool) error {
	f.activeUserID, f.active = userID, active
	return f.activeErr
}
func (f *fakeManagement) ReplaceUserRoles(_ context.Context, _ string, roleIDs []string) error {
	f.userRoleIDs = append([]string(nil), roleIDs...)
	return f.userRolesErr
}
func (f *fakeManagement) Role(context.Context, string) (RoleDetail, error) { return f.role, f.roleErr }
func (f *fakeManagement) CreateRole(_ context.Context, input RoleInput) (Role, error) {
	f.createdRole = input
	return Role{ID: "role-1", Name: input.Name, Description: input.Description}, f.roleErr
}
func (f *fakeManagement) UpdateRole(_ context.Context, _ string, input RoleInput) (Role, error) {
	f.updatedRole = input
	return Role{ID: "role-1", Name: input.Name, Description: input.Description}, f.roleErr
}
func (f *fakeManagement) DeleteRole(_ context.Context, roleID string) error {
	f.deletedRoleID = roleID
	return f.roleErr
}
func (f *fakeManagement) ReplaceRolePermissions(_ context.Context, _ string, permissionIDs []string) error {
	f.permissionIDs = append([]string(nil), permissionIDs...)
	return f.permissionsErr
}

func TestManagementMutationsNormalizeInputsAndPreserveSafetyGuards(t *testing.T) {
	users := &fakeUsers{byID: map[string]User{
		"administrator": {ID: "administrator", Active: true},
		"target":        {ID: "target", Active: true},
	}}
	management := &fakeManagement{updatedUser: User{ID: "target", Active: true}}
	jwt, _ := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	service := NewServiceWithManagement(users, nil, nil, management, &recordingPasswords{}, jwt, NewRefreshStore(&memoryRefreshCache{}, "iam:test", time.Hour))

	if _, err := service.UpdateUser(context.Background(), "target", UpdateUserInput{Email: " TARGET@EXAMPLE.COM ", Username: " target ", DisplayName: " Target "}); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if management.updatedInput.Email != "target@example.com" || management.updatedInput.Username != "target" || management.updatedInput.DisplayName != "Target" {
		t.Fatalf("normalized update = %+v", management.updatedInput)
	}
	if err := service.ResetUserPassword(context.Background(), "target", "new-password-123"); err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	if management.passwordHash != "hash" {
		t.Fatalf("stored password hash = %q", management.passwordHash)
	}

	self := WithSubject(context.Background(), "administrator")
	if err := service.SetUserActive(self, "administrator", false); !errors.Is(err, ErrCannotDeactivateSelf) {
		t.Fatalf("SetUserActive(self) error = %v", err)
	}
	management.activeErr = ErrLastActiveAdministrator
	if err := service.SetUserActive(context.Background(), "target", false); !errors.Is(err, ErrLastActiveAdministrator) {
		t.Fatalf("SetUserActive(last admin) error = %v", err)
	}
	if management.activeUserID != "target" || management.active {
		t.Fatalf("active mutation = user:%q active:%t", management.activeUserID, management.active)
	}

	if err := service.ReplaceUserRoles(context.Background(), "target", []string{"role-a", "role-b"}); err != nil {
		t.Fatalf("ReplaceUserRoles() error = %v", err)
	}
	if got := strings.Join(management.userRoleIDs, ","); got != "role-a,role-b" {
		t.Fatalf("role IDs = %q", got)
	}
}

func newTestAuth(t *testing.T, users UserRepository) *Service {
	t.Helper()
	jwt, err := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(users, nil, NewPasswordHasher(DefaultArgon2Params()), jwt, NewRefreshStore(&memoryRefreshCache{}, "iam:test", time.Hour))
}

func mapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
