# gcache
Memcache for [gin web framework](https://github.com/gin-gonic/gin)

E.g.:

- Create an API cache adding the middleware to your route:
```go
router.POST("/cache/list", gcache.Intercept(time.Hour*24, func(c *gin.Context) {
    // handler implementation		
}))
