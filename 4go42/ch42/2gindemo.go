package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
	"work/golang-test/4go42/ch42/handler"
	"work/golang-test/4go42/ch42/api"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"

	"github.com/gin-gonic/gin"
)

func Cors() gin.HandlerFunc  {
	return func(c *gin.Context) {
		c.Writer.Header().Add("Access-Control-Allow-Origin", "*")
		c.Next()
	}
}

func LimitHandler(lmt *limiter.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		httpError := tollbooth.LimitByRequest(lmt, c.Writer, c.Request)
		if httpError != nil {
			c.Data(httpError.StatusCode, lmt.GetMessageContentType(), []byte(httpError.Message))
			c.Abort()
		} else {
			c.Next()
		}
	}
}
func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.StaticFS("/public", http.Dir("C:/Code/Golang/src/work/golang-test/4go42/ch42/website/static"))

	router.LoadHTMLGlob("C:/Code/Golang/src/work/golang-test/4go42/ch42/website/tpl/*/*")
	v := router.Group("/")
	{
		v.GET("/index.html", handler.IndexHandler)
		v.GET("/add.html", handler.AddHandler)
		v.POST("/postme.html", handler.PostmeHandler)
	}

	router.Use(Cors())
	//router.Run(":8080")
	lmt:=tollbooth.NewLimiter(1,nil)
	lmt.SetMessage("server 503,try later")

	v1:=router.Group("/v1")
	{
		//v1.User(Cors())
		//v1.GET("/user/:id/*action",Cors(),api.GetUser)
		v1.GET("/user/:id/*action", LimitHandler(lmt), api.GetUser)
		// v1.OPTIONS("/users", OptionsUser)      // POST
		// v1.OPTIONS("/users/:id", OptionsUser)  // PUT, DELETE
	}

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("shutdown server..")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("server shutdown:", err)
	}
	select {
	case <-ctx.Done():
	}
	log.Println("server exiting")
}
