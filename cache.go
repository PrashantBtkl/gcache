package gcache

import (
	"errors"
	"json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	logger "github.com/sirupsen/logrus"
)

var (
	memoryCache *memCache
)

func init() {
	memoryCache = &memCache{cache: cache.New(5*time.Minute, 5*time.Minute)}
}

type memCache struct {
	sync.RWMutex
	cache *cache.Cache
}

type cacheResponse struct {
	Status int
	Header http.Header
	Data   []byte
}

var _ gin.ResponseWriter = &cachedWriter{}

// CacheIntercept encapsulates a gin handler function and caches the response with an expiration time.
func CacheIntercept(expiration time.Duration, handle gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer c.Next()
		key := generateKey(c)
		mc, err := memoryCache.getCache(key)
		if err != nil || mc.Data == nil {
			writer := newCachedWriter(expiration, c.Writer, key)
			c.Writer = writer
			handle(c)

			if c.IsAborted() {
				memoryCache.deleteCache(key)
			}
			return
		}

		c.Writer.WriteHeader(mc.Status)
		for k, vals := range mc.Header {
			for _, v := range vals {
				c.Writer.Header().Set(k, v)
			}
		}
		_, err = c.Writer.Write(mc.Data)
		if err != nil {
			memoryCache.deleteCache(key)
			logger.Error(err, "cannot write data", mc)
		}
	}
}

type cachedWriter struct {
	gin.ResponseWriter
	status  int
	written bool
	expire  time.Duration
	key     string
}

// WriteHeader satisfy the built-in interface for writers.
func (w *cachedWriter) WriteHeader(code int) {
	w.status = code
	w.written = true
	w.ResponseWriter.WriteHeader(code)
}

// Status satisfy the built-in interface for writers.
func (w *cachedWriter) Status() int {
	return w.ResponseWriter.Status()
}

// Written satisfy the built-in interface for writers.
func (w *cachedWriter) Written() bool {
	return w.ResponseWriter.Written()
}

func (w *cachedWriter) Write(data []byte) (int, error) {
	ret, err := w.ResponseWriter.Write(data)
	if err != nil {
		return 0, errors.New("fail to cache write string")
	}
	if w.Status() != 200 {
		return 0, errors.New("Write: invalid cache status")
	}
	val := cacheResponse{
		w.Status(),
		w.Header(),
		data,
	}
	b, err := json.Marshal(val)
	if err != nil {
		return 0, errors.New("validator cache: failed to marshal cache object")
	}
	memoryCache.cache.Set(w.key, b, w.expire)
	return ret, nil
}

// WriteString satisfy the built-in interface for writers.
func (w *cachedWriter) WriteString(data string) (n int, err error) {
	ret, err := w.ResponseWriter.WriteString(data)
	if err != nil {
		return 0, errors.New("fail to cache write string :" + err.Error())
	}
	if w.Status() != 200 {
		return 0, errors.New("WriteString: invalid cache status", errors.Params{"data": data})
	}
	val := cacheResponse{
		w.Status(),
		w.Header(),
		[]byte(data),
	}
	b, err := json.Marshal(val)
	if err != nil {
		return 0, errors.New("validator cache: failed to marshal cache object")
	}
	memoryCache.setCache(w.key, b, w.expire)
	return ret, err
}
