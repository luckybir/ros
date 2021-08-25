package route

import (
	"bytes"
	"context"
	"github.com/gin-contrib/pprof"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"ros/pkg/config"
	"strconv"
	"syscall"
	"time"
)

type SapOpenApiReturn struct {
	Success          string `json:"SUCCESS"`
	ErrorCode        string `json:"ERROR_CODE"`
	ErrorMessage     string `json:"ERROR_MESSAGE"`
	ExceptionCode    string `json:"EXCEPTION_CODE"`
	ExceptionMessage string `json:"EXCEPTION_MESSAGE"`
	ExceptionStack   string `json:"EXCEPTION_STACK"`
	ApiStatus        string `json:"API_STATUS"`
	LogID            string `json:"LOG_ID"`
}

type AsyncSapOpenApiHttpRequestInfo struct {
	path   string
	method string
	header http.Header
	body   []byte
}

type AsyncSapOpenApiHttpResponseInfo struct {
	body       []byte
	statusCode int
}

func InitRoute() {
	gin.SetMode(gin.ReleaseMode)
	route := gin.New()
	route.Use(zapLog(zap.L()))
	route.Use(ginzap.RecoveryWithZap(zap.L(), false))

	pprof.Register(route)

	route.GET("/ping", pingGet)
	asyncSapRouteInit(route)
	route.NoRoute(redirectSapOpenAPI)

	srv := &http.Server{
		Addr:    ":80",
		Handler: route,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.S().Fatalf("server listen and serve err:%s", err.Error())
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zap.L().Warn("Shutdown server...")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		zap.S().Fatalf("Server shutdown err: %s", err.Error())
	}

	zap.L().Warn("Server exiting")
}

func zapLog(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		// some evil middle wares modify this values
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		if len(c.Errors) > 0 {
			// Append error field if this is an erroneous request.
			for _, e := range c.Errors.Errors() {
				logger.Error(e)
			}
		} else {
			logger.Info(strconv.Itoa(c.Writer.Status()),
				zap.Int("status", c.Writer.Status()),
				zap.String("method", c.Request.Method),
				zap.String("path", path),
				zap.String("query", query),
				zap.String("ip", c.ClientIP()),
				//zap.String("user-agent", c.Request.UserAgent()),
				zap.Duration("latency", latency),
			)
		}
	}
}

func asyncSapRouteInit(route *gin.Engine) {
	for _, path := range config.ServerConfig.AsyncSapRoute {
		route.POST(path, asyncSapOpenApiPost)
	}

}

func pingGet(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"pong": time.Now().String()})
}

func asyncSapOpenApiPost(c *gin.Context) {
	ctx := c.Request.Context()

	requestBody, err := ioutil.ReadAll(c.Request.Body)

	if err != nil {
		sapOpenApiReturn := &SapOpenApiReturn{
			Success:      "false",
			ErrorMessage: err.Error(),
		}
		c.JSON(http.StatusOK, sapOpenApiReturn)
		return
	}

	//construct async sap api
	requestInfo := &AsyncSapOpenApiHttpRequestInfo{
		path:   c.Request.URL.Path,
		method: c.Request.Method,
		header: c.Request.Header,
		body:   requestBody,
	}

	requestInfo.header.Set("x-source-host", c.Request.Host)

	ch := make(chan AsyncSapOpenApiHttpResponseInfo)
	chClose := make(chan struct{})

	go func(ch chan AsyncSapOpenApiHttpResponseInfo, chClose chan struct{}, requestInfo *AsyncSapOpenApiHttpRequestInfo) {

		responseInfo := asyncToSapOpenApi(requestInfo)

		select {
		case <-chClose:
		default:
			ch <- responseInfo
		}

	}(ch, chClose, requestInfo)

	select {
	case asyncResponseInfo := <-ch:
		c.Writer.Header().Set("Content-Type", "application/json")
		c.String(asyncResponseInfo.statusCode, string(asyncResponseInfo.body))
	//case <-time.After(60 * time.Second):
	//	//fmt.Println("time out")
	//	sapOpenApiReturn := &SapOpenApiReturn{
	//		Success:      "false",
	//		ErrorMessage: "processed asynchronously when time out",
	//	}
	//	c.JSON(http.StatusOK, sapOpenApiReturn)
	case <-ctx.Done():

	}

	close(ch)
	close(chClose)

}

func asyncToSapOpenApi(requestInfo *AsyncSapOpenApiHttpRequestInfo) (responseInfo AsyncSapOpenApiHttpResponseInfo) {

	client := &http.Client{}

	url := config.ServerConfig.AsyncHost + requestInfo.path

	req, err := http.NewRequest(requestInfo.method, url, bytes.NewReader(requestInfo.body))
	if err != nil {
		zap.L().Error(err.Error())
		sapOpenApiReturn := &SapOpenApiReturn{
			Success:      "false",
			ErrorMessage: err.Error(),
		}

		responseInfo.body, _ = jsoniter.Marshal(sapOpenApiReturn)
		responseInfo.statusCode = http.StatusInternalServerError
		return
	}

	req.Header = requestInfo.header
	resp, err := client.Do(req)
	if err != nil {
		if resp == nil {

			zap.L().Error(err.Error(), zap.String("resp", "nil"))
		} else {
			zap.L().Error(err.Error(), zap.Int("statusCode", resp.StatusCode))
		}

		sapOpenApiReturn := &SapOpenApiReturn{
			Success:      "false",
			ErrorMessage: err.Error(),
		}
		responseInfo.body, _ = jsoniter.Marshal(sapOpenApiReturn)
		responseInfo.statusCode = http.StatusInternalServerError
		return
	}

	defer resp.Body.Close()

	responseInfo.body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		zap.L().Error(err.Error())
		sapOpenApiReturn := &SapOpenApiReturn{
			Success:      "false",
			ErrorMessage: err.Error(),
		}
		responseInfo.body, _ = jsoniter.Marshal(sapOpenApiReturn)
		responseInfo.statusCode = http.StatusInternalServerError
		return
	}

	responseInfo.statusCode = resp.StatusCode
	return

}

func redirectSapOpenAPI(c *gin.Context) {
	redirectURL := config.ServerConfig.AsyncHost + c.Request.URL.Path
	c.Redirect(http.StatusMovedPermanently, redirectURL)
}
