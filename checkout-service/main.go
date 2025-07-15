package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type CheckoutItem struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

func main() {
	e := echo.New()

	e.POST("/checkout", handleCheckout)

	e.Logger.Fatal(e.Start(":3005"))
}

func handleCheckout(c echo.Context) error {
	var items []CheckoutItem
	if err := c.Bind(&items); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid checkout data")
	}


	return c.JSON(http.StatusOK, "Checkout successful")
}
