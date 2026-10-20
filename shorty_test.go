package shorty

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	_ "github.com/lib/pq"

)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
  gin.SetMode(gin.TestMode)
  os.Exit(m.Run())
}

func newShorty() *Shorty {
	return New(getTestDB(), NewSynchronizedGenerator(NewBasicGenerator(Alphanumeric, 6)), nil)
}

func TestAddWithProperData(t *testing.T) {
	db := getTestDB()
	shorty := newShorty()
	t.Cleanup(resetDB)
	eng := gin.New()
	eng.POST("/add", shorty.Add)
	tc := testClient{eng}

	url := "http://google.com"
	w := tc.Post("/add", map[string]any{"url": url})

	assert.Equalf(t, 201, w.Code, "invalid response code")

	var link Link = Link{Code: "qqqqqq", URL: url}
	var dburl string
	db.QueryRow("SELECT url from links;").Scan(&dburl)
	assertResponseBody(t, link, w)
	assert.Equal(t, url, link.URL)
	assert.Equal(t, dburl, link.URL)
}

func TestAddWithExistingURL(t *testing.T) {
	db := getTestDB()
	shorty := newShorty()
	t.Cleanup(resetDB)
	eng := gin.New()
	eng.POST("/add", shorty.Add)
	tc := testClient{eng}
	payload := map[string]any{"url": "http://google.com"}

	tc.Post("/add", payload)
	response := tc.Post("/add", payload)

	var total_records int
	_ = db.QueryRow("SELECT count(*) from links;").Scan(&total_records)
	assert.Equal(t, 200, response.Code)
	assert.Equal(t, 1, total_records)
}

func TestAddWithExhaustedGenerator(t *testing.T) {
	t.Cleanup(resetDB)
	shorty := newShorty()
	gen := NewBasicGenerator("AB", 2)
	gen.exhausted = true
	shorty.SetGenerator(gen)
	eng := gin.New()
	eng.POST("/add", shorty.Add)
	payload := map[string]any{"url": "http://google.com"}
	tc := testClient{eng}

	w := tc.Post("/add", payload)

	assert.Equalf(t, 409, w.Code, "Response code does not match")
	assertResponseBody(t, CodesTakenResponse, w)
}

func TestAddWithPopulatedDatabase(t *testing.T) {
	t.Cleanup(resetDB)
	db := getTestDB()
	shorty := newShorty()
	stmt := "INSERT INTO links (code, url) VALUES (:code, :url)"
	eng := gin.New()
	eng.POST("/add", shorty.Add)
	db.NamedExec(stmt, []map[string]any{
		{"code": "qqqqqq", "url": "https://google.com"},
		{"code": "qqqqqw", "url": "https://reddit.com"},
	})
	url := "http://twitter.com"
	payload := map[string]any{"url": url}
	tc := testClient{eng}

	w := tc.Post("/add", payload)

	assert.Equal(t, 201, w.Code)
	var link Link = Link{Code: "qqqqqf", URL: url}
	var dburl string
	var count int
	db.QueryRow("SELECT url from links WHERE code=$1;", link.Code).Scan(&dburl)
	db.QueryRow("SELECT COUNT(*) from links;").Scan(&count)

	assertResponseBody(t, link, w)
	assert.Equal(t, 3, count)
	assert.Equal(t, url, link.URL)
	assert.Equal(t, dburl, link.URL)
}

func TestGetWithExistingCode(t *testing.T) {
	t.Cleanup(resetDB)
	db := getTestDB()
	shorty := newShorty()
	eng := gin.New()
	eng.GET("/:code", shorty.Get)
	tc := testClient{eng}
	stmt := "INSERT INTO links (code, url) VALUES ($1, $2)"
	code := "aaaaaa"
	url := "https://google.com"
	db.MustExec(stmt, code, url)

	w := tc.Get(fmt.Sprintf("/%s", code))

	if !assert.Equal(t, 307, w.Code) { assert.FailNow(t, "Expected redirect") }
	assert.Equal(t, url, w.Header()["Location"][0])
}

func TestGetWithNonExistingCode(t *testing.T) {
	t.Cleanup(resetDB)
	shorty := newShorty()
	eng := gin.New()
	eng.GET("/:code", shorty.Get)
	tc := testClient{eng}
	code := "aaaaaa"
	expected := NotFoundResponse

	w := tc.Get(fmt.Sprintf("/%s", code))

	assert.Equal(t, 404, w.Code)
	assertResponseBody(t, expected, w)
}

func getTestDB() *sqlx.DB {
	if testDB == nil {
		conn := "sslmode=disable host=localhost user=postgres password=postgres dbname=shorty"
		var err error
		testDB, err = sqlx.Open("postgres", conn)
		if err != nil {
			panic("Could not connect to test database")
		}
	}
	return testDB
}

func resetDB() {
	testDB.MustExec("DELETE FROM links WHERE id > 0")
}

func assertResponseBody(t *testing.T, want any, got *httptest.ResponseRecorder) {
	byte_data, _ := json.Marshal(want)
	expectedResponse := string(byte_data)
	body := got.Body.String()

	assert.Equal(t, expectedResponse, body)
}


type testClient struct {
	eng *gin.Engine
}

func (tc testClient) Post(url string, data map[string]any) *httptest.ResponseRecorder {
	var request *http.Request
	w := httptest.NewRecorder()
	if data != nil {
		js, _ := json.Marshal(data)
		request, _ = http.NewRequest("POST", url, strings.NewReader(string(js)))
	} else {
		request, _ = http.NewRequest("POST", url, nil)
	}
	tc.eng.ServeHTTP(w, request)
	return w
}

func (tc testClient) Get(url string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", url, nil)
	tc.eng.ServeHTTP(w, request)
	return w
}
