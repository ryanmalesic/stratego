package main

import (
	"context"
	"lambda/games"
	"lambda/utils"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	"github.com/gin-gonic/gin"
)

var ginLambda *ginadapter.GinLambda
var gs *games.GamesService

func getGin() *gin.Engine {
	r := gin.Default()

	if !utils.InLambda() {
		os.Setenv("STORAGE_STRATEGO_NAME", "Stratego-dev")
		r.Use(func(c *gin.Context) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Amz-Security-Token, x-amz-date")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}

			c.Next()
		})
	}

	r.POST("/games", gs.CreateGame)
	r.POST("/games/:id", gs.JoinGame)
	r.POST("/games/:id/moves", gs.Move)

	return r
}

func init() {
	gs = games.NewGamesService()
	ginLambda = ginadapter.New(getGin())
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return ginLambda.ProxyWithContext(ctx, req)
}

func main() {
	if utils.InLambda() {
		lambda.Start(Handler)
	} else {
		http.ListenAndServe(":8080", getGin())
	}
}
