package main

// import (
// 	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/worker/endpoints"
// 	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/state"

// 	"github.com/gofiber/fiber/v2"
// 	"github.com/gofiber/fiber/v2/middleware/logger"
// )

// func NewServer(globalState *state.GlobalState) *fiber.App {
// 	app := fiber.New(fiber.Config{
// 		ReadBufferSize:  8192 * 1024,
// 		WriteBufferSize: 8192 * 1024,
// 		BodyLimit:       10 * 1024 * 1024,
// 	})

// 	app.Use(logger.New(logger.Config{
// 		Format: "[${time}] ${status} - ${method} ${path} ${latency}\n",
// 	}))

// 	app.Get("/server-name", endpoints.GetServerName(globalState))
// 	app.Get("/functions", endpoints.ListFunctions(globalState))
// 	app.Post("/functions/:functionName/:version/execute", endpoints.ExecuteFunction(globalState))
// 	app.Get("/functions/:functionName/:version/definition", endpoints.GetFunctionDefinition(globalState))
// 	return app
// }
