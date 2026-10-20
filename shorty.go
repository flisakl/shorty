package shorty

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type Shorty struct {
	db        *sqlx.DB
	generator Generator
	translator ut.Translator
}

func New(database *sqlx.DB, g Generator, trans ut.Translator) *Shorty {
	s := &Shorty{db: database, translator: trans}
	s.SetGenerator(g)
	return s
}

type ErrorResponse struct {
	Code        string              `json:"code"`
	Message     string              `json:"message"`
	FieldErrors map[string]string `json:"field_errors,omitempty"`
}

var BadJsonResponse = ErrorResponse{Code: "BAD_JSON", Message: "Submitted JSON could not be parsed"}
var CodesTakenResponse = ErrorResponse{Code: "NO_CODE_AVAILABLE", Message: "All codes are taken, try another domain." }
var NotFoundResponse = ErrorResponse{Code: "NOT_FOUND", Message: "Resource could not be found"}
var TryLaterResponse = ErrorResponse{Code: "TRY_AGAIN_LATER", Message: "Server error, try again later"}

type Link struct {
	Code string `json:"code"`
	URL  string `json:"url"`
}


func (s *Shorty) Add(c *gin.Context) {

	type input struct {
		URL string `json:"url" binding:"required,http_url"`
	}

	// check for errors
	var url input
	err := c.ShouldBindJSON(&url)
	ctx := c.Request.Context()

	if err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			response := ErrorResponse{
				Code: "VALIDATION_FAILED",
				Message: "Validation failed on one of the fields",
				FieldErrors: ve.Translate(s.translator),
			}
			c.JSON(http.StatusBadRequest, response)
			return
		}
		c.JSON(http.StatusBadRequest, BadJsonResponse)
		return
	}

	// check if url already exists in db
	var link Link
	err = s.db.QueryRowxContext(ctx, "SELECT code, url FROM links WHERE url=$1", url.URL).StructScan(&link)

	// if record exists, return it to user
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"code": link.Code, "url": link.URL})
		return
	}

	// check if we have available codes
	code := s.generator.Get()
	if code == "" {
		c.JSON(http.StatusConflict, CodesTakenResponse)
		return
	}

	link = Link{Code: code, URL: url.URL}
	err = s.insert(ctx, link.Code, link.URL)

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "23505" {
		s.UpdateGenerator()
		code := s.generator.Get()
		if code == "" {
			c.JSON(http.StatusConflict, CodesTakenResponse)
			return
		}
		link.Code = code
		err = s.insert(ctx, link.Code, link.URL)
	}

	if err == nil {
		c.JSON(http.StatusCreated, gin.H{"code": link.Code, "url": link.URL})
		return
	}
	c.JSON(http.StatusServiceUnavailable, TryLaterResponse)
}

func (s *Shorty) insert(ctx context.Context, code, url string) error {
	stmt := "INSERT INTO links (code, url) VALUES ($1, $2) RETURNING id;"
	_, err := s.db.ExecContext(ctx, stmt, code, url)
	return err
}

func (s *Shorty) Get(c *gin.Context) {
	code := c.Param("code")
	stmt := "SELECT url FROM links WHERE code=$1"
	var urlToRedirect string
	err := s.db.QueryRow(stmt, code).Scan(&urlToRedirect)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, NotFoundResponse)
		return
	} else if err != nil {
		c.JSON(http.StatusServiceUnavailable, TryLaterResponse)
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, urlToRedirect)
}

func (s *Shorty) SetGenerator(g Generator) {
	if _, ok := g.(*SynchronizedGenerator); ok {
		s.generator = g
	} else {
		s.generator = NewSynchronizedGenerator(g)
	}
}

// UpdateGenerator updates generator with code from last row
func (s *Shorty) UpdateGenerator() {
	var link Link
	stmt := "SELECT code, url FROM links ORDER BY id DESC LIMIT 1;"
	err := s.db.Get(&link, stmt)
	if err == nil {
		s.generator.Update(link.Code)
		return
	}
}
