package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func main() {
	initDb()
	defer db.Close()

	e := echo.New()

	e.Use(middleware.Recover())

	// Routes
	e.GET("", getProducts)
	e.GET("/:id", getProduct)
	e.POST("", addProduct)
	e.PUT("/:id", updateProduct)
	e.DELETE("/:id", deleteProduct)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	// Start server
	e.Logger.Fatal(e.Start(":" + port))
}

func initDb() {
	var err error
	// Fix the typo in "sqlite3"
	db, err = sql.Open("sqlite3", "./products.db")
	if err != nil {
		log.Fatal(err.Error())
	}

	createTableQuery := `CREATE TABLE IF NOT EXISTS products(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		price REAL NOT NULL);`

	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatal(err)
	}

	// Clear and seed initial data (optional, for demonstration)
	_, err = db.Exec("DELETE FROM products")
	if err != nil {
		log.Fatal(err)
	}

	// Use parameterized queries to insert initial data
	_, err = db.Exec("INSERT INTO products(name, price) VALUES(?, ?)", "Laptop", 1000)
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO products(name, price) VALUES(?, ?)", "Phone", 500)
	if err != nil {
		log.Fatal(err)
	}
}

func getProducts(c echo.Context) error {
	rows, err := db.Query("SELECT id, name, price FROM products")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		products = append(products, p)
	}

	return c.JSON(http.StatusOK, products)
}

func getProduct(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid product id")
	}

	var p Product
	err = db.QueryRow("SELECT id, name, price FROM products WHERE id = ?", id).Scan(&p.ID, &p.Name, &p.Price)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, "Product not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, p)
}

func addProduct(c echo.Context) error {
	var p Product
	if err := c.Bind(&p); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	result, err := db.Exec("INSERT INTO products(name, price) VALUES(?, ?)", p.Name, p.Price)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	id, err := result.LastInsertId()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	p.ID = int(id)
	return c.JSON(http.StatusCreated, p)
}

func updateProduct(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid product id")
	}

	var p Product
	if err := c.Bind(&p); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	result, err := db.Exec("UPDATE products SET name = ?, price = ? WHERE id = ?", p.Name, p.Price, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Product not found")
	}

	p.ID = id
	return c.JSON(http.StatusOK, p)
}

func deleteProduct(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid product id")
	}

	result, err := db.Exec("DELETE FROM products WHERE id = ?", id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Product not found")
	}

	return c.NoContent(http.StatusNoContent)
}
