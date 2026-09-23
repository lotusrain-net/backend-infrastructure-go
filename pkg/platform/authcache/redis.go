package authcache

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"github.com/redis/go-redis/v9"
)

type Store struct {
	client redis.UniversalClient
	prefix string
	pepper []byte
}

func New(client redis.UniversalClient, environment string, pepper []byte) *Store {
	return &Store{client: client, prefix: "auth:" + environment + ":", pepper: append([]byte(nil), pepper...)}
}
func (s *Store) digest(value string) string {
	h := hmac.New(sha256.New, s.pepper)
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}
func (s *Store) emailKey(email, purpose string) string {
	return s.prefix + "email:" + purpose + ":" + s.digest(strings.ToLower(strings.TrimSpace(email)))
}
func randomID() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}

const issueScript = `
local retry=0
for i=3,4 do
 local count=redis.call('INCR',KEYS[i])
 if count==1 then redis.call('EXPIRE',KEYS[i],3600) end
 local limit=tonumber(ARGV[i])
 if count>limit then retry=math.max(retry,math.ceil(redis.call('PTTL',KEYS[i])/1000)) end
end
retry=math.max(retry,math.ceil(redis.call('PTTL',KEYS[2])/1000))
if retry>0 then return retry end
redis.call('SET',KEYS[2],'1','EX',60)
redis.call('HSET',KEYS[1],'digest',ARGV[1],'id',ARGV[2],'errors',0)
redis.call('EXPIRE',KEYS[1],600)
return 0`

func (s *Store) Issue(ctx context.Context, email, purpose, ip string) (string, error) {
	n, e := rand.Int(rand.Reader, big.NewInt(1000000))
	if e != nil {
		return "", e
	}
	code := fmt.Sprintf("%06d", n.Int64())
	id, e := randomID()
	if e != nil {
		return "", e
	}
	key := s.emailKey(email, purpose)
	retry, e := s.client.Eval(ctx, issueScript, []string{key, key + ":cooldown", s.prefix + "limit:email:" + s.digest(strings.ToLower(strings.TrimSpace(email))), s.prefix + "limit:ip:" + s.digest(ip)}, s.digest(key+":"+code), id, 10, 50).Int()
	if e != nil {
		return "", e
	}
	if retry > 0 {
		return "", &iam.RateLimitError{RetryAfter: retry}
	}
	return code, nil
}

const verifyScript = `
if redis.call('EXISTS',KEYS[1])==0 then return {} end
if tonumber(redis.call('HGET',KEYS[1],'errors'))>=5 then return {} end
if redis.call('HGET',KEYS[1],'digest')~=ARGV[1] then
 redis.call('HINCRBY',KEYS[1],'errors',1)
 return {}
end
return {redis.call('HGET',KEYS[1],'id'),tostring(redis.call('PTTL',KEYS[1]))}`

func (s *Store) Verify(ctx context.Context, email, purpose, code string) (iam.CodeReceipt, error) {
	key := s.emailKey(email, purpose)
	started := time.Now()
	v, e := s.client.Eval(ctx, verifyScript, []string{key}, s.digest(key+":"+code)).StringSlice()
	if e != nil {
		return iam.CodeReceipt{}, e
	}
	if len(v) != 2 {
		return iam.CodeReceipt{}, iam.ErrInvalidCode
	}
	ttl, e := strconv.ParseInt(v[1], 10, 64)
	if e != nil || ttl <= 0 {
		return iam.CodeReceipt{}, iam.ErrInvalidCode
	}
	return iam.CodeReceipt{ID: v[0], ExpiresAt: started.Add(time.Duration(ttl) * time.Millisecond)}, nil
}

const consumeScript = `
if redis.call('HGET',KEYS[1],'id')~=ARGV[1] then return 0 end
redis.call('DEL',KEYS[1]);return 1`

func (s *Store) Consume(ctx context.Context, email, purpose string, receipt iam.CodeReceipt) error {
	n, e := s.client.Eval(ctx, consumeScript, []string{s.emailKey(email, purpose)}, receipt.ID).Int()
	if e != nil {
		return e
	}
	if n != 1 {
		return iam.ErrInvalidCode
	}
	return nil
}

type challengeStore struct{ s *Store }

func (s *Store) Challenges() iam.Challenges   { return challengeStore{s} }
func (c challengeStore) key(id string) string { return c.s.prefix + "challenge:" + c.s.digest(id) }
func (c challengeStore) Issue(ctx context.Context, v iam.Challenge) (string, error) {
	id, e := randomID()
	if e != nil {
		return "", e
	}
	e = c.s.client.Eval(ctx, `redis.call('HSET',KEYS[1],'user',ARGV[1],'version',ARGV[2],'attempts',0);redis.call('EXPIRE',KEYS[1],300);return 1`, []string{c.key(id)}, v.UserID, v.Version).Err()
	return id, e
}
func (c challengeStore) Attempt(ctx context.Context, id string) (iam.Challenge, error) {
	if len(id) != 64 {
		return iam.Challenge{}, iam.ErrInvalidCode
	}
	v, e := c.s.client.Eval(ctx, `if redis.call('EXISTS',KEYS[1])==0 then return {} end; if redis.call('HINCRBY',KEYS[1],'attempts',1)>5 then return {} end;return {redis.call('HGET',KEYS[1],'user'),redis.call('HGET',KEYS[1],'version')}`, []string{c.key(id)}).StringSlice()
	if e != nil {
		return iam.Challenge{}, e
	}
	if len(v) != 2 {
		return iam.Challenge{}, iam.ErrInvalidCode
	}
	version, e := strconv.ParseInt(v[1], 10, 64)
	return iam.Challenge{UserID: v[0], Version: version}, e
}
func (c challengeStore) Consume(ctx context.Context, id string) error {
	n, e := c.s.client.Del(ctx, c.key(id)).Result()
	if e != nil {
		return e
	}
	if n != 1 {
		return iam.ErrInvalidCode
	}
	return nil
}
