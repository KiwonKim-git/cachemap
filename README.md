# cachemap
This repository is to provide cache by using hashmap written in GO

## 1. cachemap with sync.Map

### 1.1. import library 

``` go
import "github.com/KiwonKim-git/cachemap/cache"
```

### 1.2. define expiration duration of element in cachemap
Duration allows the type of [time.Duration](https://golang.org/pkg/time/) and duration should be greater than 60 seconds.  
If it is less than 60 seconds, the duration will be set as 60 seconds  

exmaple : 

``` go    
const DURATION time.Duration = time.Hour * 96 // 4 days
```

### 1.3. define schedule time to remove expired element in cachemap
The expression follows cron expression. Based on the schedule expression, cachemap will remove all expired elements throughout the cache.  

example : 

``` go
const SCHEDULE string = "0 0 18 * * *"        // run at everyday 18:00 in UTC
```

### 1.4. create cachemap 
if the Verbose is true, this liberary will print logs when there is load and store operation.   
If you set up scheduler, you can add function for pre-processing and post-processing. (Default : nil)    
PreProcess will be called before deletion of entry in cache and PostProcess will be call after deletion of entry in cache.   

example :    

``` go
tokenCache = cache.NewCacheMap(&schema.CacheConf{
    Verbose:              true,
    Name:                 "TokenCache",
    CacheDuration:        DURATION,
    RandomizedDuration:   false,
    RedisConf:            nil,
    SchedulerConf: &schema.SchedulerConf{
        CronExprForScheduler: SCHEDULE,
        PreProcess:           func(value interface{}) (logs string, err error){return "", nil},
        PostProcess:          func(value interface{}) (logs string, err error){return "", nil},
    },
})
```

Regarding the variable "RandomizedDuration" in the CacheConf, you can choose the option to randomize expiry time if the value is true.  
Randomized duration will distribute cache update events more evenly.

### 1.5. store key/value
Expiry time will be decided when the element is stored in the cache.  
If the "randomizedDuration" is true, duration will be randomized up to 25% of "DURATION".  
If you want to set specific expiry time, you can set expireAt value with the expiry time.  
Of course, expireAt should be after now.  

example :    

```go
tokenCache.Store(tokenCacheKey, token, nil)
```

### 1.6. load key/value

example :     

``` go
element, status, lastUpdated := tokenCache.Load(tokenCacheKey)
```

You can test whether the value is valid or not. "status" will be one of the values below
> VALID : found value by the key and it is not expired yet.   
> EXPIRED : found value by the key but, it was already expired.   
> NOT_FOUND : can not get the value by the key

```go
const (
    VALID RESULT = iota
    EXPIRED
    NOT_FOUND
)
```

## 2. cachemap with Redis

### 2.1. import library 

``` go
import "github.com/KiwonKim-git/cachemap/cacheRedis"
```

### 2.2. define expiration duration of element in cachemap
Duration allows the type of [time.Duration](https://golang.org/pkg/time/) and duration should be greater than 60 seconds.  
If it is less than 60 seconds, the duration will be set as 60 seconds  

exmaple :     

``` go    
const DURATION time.Duration = time.Hour * 96 // 4 days
```

### 2.3. define schedule time to remove expired element in cachemap
The cache for Redis does not use this feature because removing expired entries will be handled by Redis.    

### 2.4. create cachemap 

if the Verbose is true, this liberary will print logs when there is load and store operation.    
The cache with Redis requires the configuration to connect and use Redis Server.  

example :    

```go
sessionCache = cacheRedis.CreateCacheRedis(ctx, &schema.CacheConf{
    Verbose:            true,
    Name:               "SessionCache",
    CacheDuration:      DURATION,
    RandomizedDuration: true,
    RedisConf: &schema.RedisConf{
        Namespace:       "session",
        Group:           "",
        ServerAddresses: []string{"HOST1_ADDRESS:PORT", "HOST2_ADDRESS:PORT", "..."},
        Username:        "REDIS_USER_NAME",
        Password:        "REDIS_PASSWORD,
    },
})
```

Regarding the variable "RandomizedDuration" in the CacheConf, you can choose the option to randomize expiry time if the value is true.  
Randomized duration will distribute cache update events more evenly.

### 2.5. store key/value
Expiry time will be decided when the element is stored in the cache.  
If the "randomizedDuration" is true, duration will be randomized up to 25% of "DURATION".  
If you want to set specific expiry time, you can set expireAt value with the expiry time.  
Of course, expireAt should be after now.  

Due to the fact that the cache with Redis needs communication with Redis Server, the Store function also returns error.    
The application should handle the case if error is not nil.  

example :    

```go
err = sessionCache.Store(ctx, sessionId, sessionContext, sessionEnd)
```

### 2.6. load key/value

Due to the fact that the cache with Redis needs communication with Redis Server, the Load function also returns error.    
The application should handle the case if error is not nil.    

example :     

``` go
element, status, lastUpdated, err := sessionCache.Load(ctx, sessionId)
```

You can test whether the value is valid or not. "status" will be one of the values below
> VALID : found value by the key and it is not expired yet.   
> EXPIRED : found value by the key but, it was already expired.   
> NOT_FOUND : can not get the value by the key

```go
const (
    VALID RESULT = iota
    EXPIRED
    NOT_FOUND
)
```

## 3. Effect when "randomizedDuration" is true.

### 3.1. Example of 'false'
As you can see image below, most of cache update is concentrated near expiration time.   
It may cause surge or peak resource consuming to handle all updates. 
- expiration duration : 96 hours (4days)  + randomizedDuration is 'false'    
![cache_random_false](https://user-images.githubusercontent.com/84881258/119936398-144cba00-bfc4-11eb-928a-523aa9502b67.png)

### 3.2. Example of 'true'
As compared with 'false', cache update is more evenly distributed through time span.
- expiration duration : 96 hours (4days) + randomizedDuration is 'true'    
![cache_random_true](https://user-images.githubusercontent.com/84881258/120125478-f1a0e800-c1f3-11eb-87b0-0c2cbc72ba5a.png)

## 4. The licenses for the rest of the 3rd party materials 
> Copyright (C) 2012 Rob Figueiredo
All Rights Reserved.

MIT LICENSE

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.   


End of Document
