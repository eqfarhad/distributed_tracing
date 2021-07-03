package people

import (
	"context"
	"database/sql"
	"log"
	"os"

	"medium-opentelemetry-poc/lib/model"

	_ "github.com/go-sql-driver/mysql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

// Repository retrieves information about people.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository backed by MySQL database.
func NewRepository() *Repository {
	dburl := getenv("MYSQL_URL", "root:mysqlpwd@tcp(127.0.0.1:3306)/sampleDB")
	log.Print("dbURL=" + dburl)
	db, err := sql.Open("mysql", dburl)
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatalf("Cannot ping the db: %v", err)
	}
	return &Repository{
		db: db,
	}
}

// GetPerson tries to find the person in the database by name.
// If not found, it still returns a Person object with only name
// field populated.
func (r *Repository) GetPerson(ctx context.Context, name string) (model.Person, error) {
	ctx, span := otel.Tracer("repository").Start(ctx, "GetPerson-function")
	defer span.End()
	span.AddEvent("Repository event!")

	query := "select title, description from people where name = ?"
	span.SetAttributes(attribute.Key("Query").String(query))

	rows, err := r.db.QueryContext(ctx, query, name)
	if err != nil {
		return model.Person{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var title, descr string
		err := rows.Scan(&title, &descr)
		if err != nil {
			return model.Person{}, err
		}
		return model.Person{
			Name:        name,
			Title:       title,
			Description: descr,
		}, nil
	}
	return model.Person{
		Name: name,
	}, nil
}

// Close calls close on the underlying db connection.
func (r *Repository) Close() {
	r.db.Close()
}
