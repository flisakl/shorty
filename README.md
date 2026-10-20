# shorty

Shorty is a simple url shortener, it provides only 2 endpoints:
1. Get - for redirecting
1. Add - for creating shorter url for given url

Currently only postgres database is supported. Migrations can be applied
using [goose](https://github.com/pressly/goose). Additional features such
as caching were not added to keep the dependencies to minimum.

## Usage

Package relies on Generator interface to provide unique code for each url. 

```go
type Generator interface {
	// Get returns an unique sequence of fixed length or empty string if generator
	// has been exhausted. 
	Get() string
	// Update changes the generator's internal state using provided string. The purpose
	// of this method is to allow generating sequence from a certain point. Error is
	// returned if operation fails.
	Update(lastSequence string) error
}
```

There is a built-in generator, you can find out how to use it below. Built-in
generator uses a set of unique characters to generate every possible
sequence of given length. Number of available sequences is determined by code
length and total number of characters in set, it can be expressed by this formula:
**numberOfCharacters**<sup>**codeLength**</sup>.


| Symbols | Number of characters | Code length | Number of possible sequences |
| ------- | -------------------- | ----------- | ---------------------------- |
| Numeric (0-9) | 10 | 6 | 1 000 000 |
| Alpha (a-z A-Z) | 52 | 6 | 19 770 609 664
| Alphanumeric (a-z A-Z 0-9) | 62 | 6 | 56 800 235 584


> [!WARNING]
> Built-in generator does not check if characters are unique or if they are allowed to use in urls.

### Simple example with translation
```go
package main

import (
	"github.com/flisakl/shorty"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	"github.com/jmoiron/sqlx"
)

func main() {

	// Set these variables
	var dbString string = "sslmode=disable host=localhost user=postgres password=postgres dbname=shorty"
	var ginPort string = ":8080"

	// Connect with postgres database
	db, err := sqlx.Open("postgres", dbString)

	if err != nil {
		panic(err)
	}

	// Setup english translation for validator package
	en := en.New()
	uni := ut.New(en, en)
	enTranslator, found := uni.GetTranslator("en")
	validator := validator.New()
	en_translations.RegisterDefaultTranslations(validator, enTranslator)

	if !found {
		panic("English translation not found")
	}

	// Create Generator and Gin engine
	generator := shorty.NewBasicGenerator(shorty.Alphanumeric, 6)
	shortener := shorty.New(db, generator, enTranslator)
  shortener.UpdateGenerator()
	gin.SetMode(gin.ReleaseMode)
	eng := gin.New()
	eng.Use(gin.Recovery())
	eng.POST("", shortener.Add)
	eng.GET(":code", shortener.Get)

	eng.Run(ginPort)
}
```
