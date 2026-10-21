package mix

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type User struct {
	ID    int    `path:"id"`
	Name  string `json:"name"`
	Age   int    `header:"age"`
	Phone string `query:"phone"`
}

func TestBind(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://localhost/user/1?phone=123", bytes.NewBufferString(`{"name":"test"}`))
	require.NoError(t, err)
	req.Pattern = "/user/{id}"
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Age", "16")

	var u User
	err = Bind(req, &u)
	require.NoError(t, err)
	// JSON body 分支会直接返回，不会继续绑定 uri/query/header。
	assert.Equal(t, 0, u.ID)
	assert.Equal(t, "test", u.Name)
	assert.Equal(t, 0, u.Age)
	assert.Equal(t, "", u.Phone)
}

type User2 struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Phone string `json:"phone"`
	User  User   `json:"user"`
}

func TestBind2(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://localhost/user/1?phone=123", bytes.NewBufferString(`{"name":"test","user":{"id":1}}`))
	require.NoError(t, err)
	req.Pattern = "/user/{id}"
	req.SetPathValue("id", "1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Age", "16")

	var u1 User2
	err = Bind(req, &u1)
	require.NoError(t, err)
	assert.Equal(t, 0, u1.ID)
	assert.Equal(t, "test", u1.Name)

	req, err = http.NewRequest(http.MethodPost, "http://localhost/user/2?phone=007", bytes.NewBufferString(`{"name":"test2"}`))
	require.NoError(t, err)
	req.Pattern = "/user/{id}"
	req.SetPathValue("id", "2")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Age", "18")

	var u2 User2
	err = Bind(req, &u2)
	require.NoError(t, err)
	assert.Equal(t, 0, u2.ID)
	assert.Equal(t, "test2", u2.Name)
}

func TestBindFormContentType_StillBindsURIQueryHeader(t *testing.T) {
	type FormUser struct {
		ID    int    `path:"id"`
		Name  string `form:"name"`
		Age   int    `header:"age"`
		Phone string `query:"phone"`
	}
	req, err := http.NewRequest(http.MethodPost, "http://localhost/user/9?phone=888", bytes.NewBufferString("name=formname"))
	require.NoError(t, err)
	req.Pattern = "/user/{id}"
	req.SetPathValue("id", "9")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Age", "21")

	var u FormUser
	err = Bind(req, &u)
	require.NoError(t, err)
	assert.Equal(t, 9, u.ID)
	assert.Equal(t, "formname", u.Name)
	assert.Equal(t, 21, u.Age)
	assert.Equal(t, "888", u.Phone)
}

func TestBindFromURIQueryHeaderWithoutBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://localhost/user/1?phone=123", nil)
	require.NoError(t, err)
	req.Pattern = "/user/{id}"
	req.SetPathValue("id", "1")
	req.Header.Set("Age", "16")

	var u User
	err = Bind(req, &u)
	require.NoError(t, err)
	assert.Equal(t, 1, u.ID)
	assert.Equal(t, 16, u.Age)
	assert.Equal(t, "123", u.Phone)
	assert.Equal(t, "", u.Name)
}

func TestHeaderSource_URLUnescapeViaGETBind(t *testing.T) {
	type HeaderUser struct {
		Token string `header:"x-token"`
		Tag   string `header:"x-tag"`
	}
	req, err := http.NewRequest(http.MethodGet, "http://localhost/ping", nil)
	require.NoError(t, err)
	req.Header.Set("X-Token", "hello%20world")
	req.Header.Set("X-Tag", "a%2Bb")

	var u HeaderUser
	err = Bind(req, &u)
	require.NoError(t, err)
	assert.Equal(t, "hello world", u.Token)
	assert.Equal(t, "a+b", u.Tag)
}

func TestHeaderSource_GetDirectly(t *testing.T) {
	hs := HeaderSource{"X-Token": {"foo%20bar"}}
	vals, ok := hs.Get("x-token")
	require.True(t, ok)
	require.Len(t, vals, 1)
	assert.Equal(t, "foo bar", vals[0])
}

// DELETE / PUT 只带路径参数、没有 body 时，空 body 不能短路掉 path/query 绑定：
// 之前这里 return，导致取消点赞、重命名/删除收藏夹都拿不到 id。
func TestBindDeleteWithoutBodyBindsPath(t *testing.T) {
	req, err := http.NewRequest(http.MethodDelete, "http://localhost/api/content/container/7", nil)
	require.NoError(t, err)
	req.Pattern = "/api/content/container/{id}"
	req.SetPathValue("id", "7")

	var v struct {
		Id uint64 `json:"id"`
	}
	require.NoError(t, Bind(req, &v))
	assert.Equal(t, uint64(7), v.Id)
}
