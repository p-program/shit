package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 中间件：日志 + traceID
func LoggerWithTraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		traceID := uuid.New().String()
		c.Set("traceID", traceID)

		// 继续执行
		c.Next()

		// 记录日志
		duration := time.Since(start)
		log.Printf("[TraceID:%s] %s %s %d %s",
			traceID,
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration,
		)
	}
}

type ShitRequest struct {
	Smoothness string
}

// Shit 路由方法
func Shit(c *gin.Context) {
	traceID, _ := c.Get("traceID")

	var req ShitRequest
	// 解析请求体 JSON 到结构体
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数：" + err.Error(),
		})
		return
	}

	// 写入 ClickHouse
	ctx := context.Background()
	err := clickhouseConn.Exec(ctx, `
		INSERT INTO shit.toilet_log (id, log_time, smoothness)
		VALUES (?, ?, ?)`,
		uuid.New().ID(), time.Now(), req.Smoothness,
	)
	if err != nil {
		log.Println("写入 ClickHouse 失败:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入数据库失败"})
		return
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"message":    "写入成功",
		"traceID":    traceID,
		"smoothness": req.Smoothness,
	})
}

func main() {

	// if !waitForPortAvailable(9000, 3, 3*time.Second) {
	// 	fmt.Errorf("click house 一直未能启动")
	// 	return
	// }
	for i := 0; i < 2; i++ {
		// 初始化 ClickHouse
		err := initClickHouse()
		if err != nil {
			time.Sleep(time.Second * 3)
		} else {
			break
		}

	}

	r := gin.Default()

	// 加载静态文件 index.html
	r.StaticFile("/", "./static/index.html")

	// 使用自定义日志+traceID中间件
	r.Use(LoggerWithTraceID())

	// 路由 /shit → 委托给 Shit 方法
	r.POST("/shit", Shit)

	// 启动服务
	r.Run(":8080") // 监听 8080 端口
}

// ClickHouse 客户端
var clickhouseConn clickhouse.Conn

func initClickHouse() error {
	var err error

	clickhouseUrl := os.Getenv("CLICKHOUSE_HOST") + ":" + os.Getenv("CLICKHOUSE_PORT")
	if clickhouseUrl == "" {
		clickhouseUrl = "127.0.0.1:9000"
	}
	// clickhouseUser := os.Getenv("CLICKHOUSE_USER")
	// clickhousePassword := os.Getenv("CLICKHOUSE_PASSWORD")
	// clickhouseDB := os.Getenv("CLICKHOUSE_DB")
	clickhouseConn, err = clickhouse.Open(&clickhouse.Options{
		Addr: []string{clickhouseUrl},
		Auth: clickhouse.Auth{
			Database: "shit",
			Username: "default",
			Password: "123456",
		},
	})

	if err != nil {
		log.Fatal("ClickHouse 连接失败:", err)
		return err
	}

	// 执行创建数据库 SQL
	createDatabaseSQL := `CREATE DATABASE IF NOT EXISTS shit;`
	var i int = 0
	for ; i < 10; i++ {
		if err := clickhouseConn.Exec(context.Background(), createDatabaseSQL); err != nil {
			log.Fatal("执行 SQL 失败:", err)
			time.Sleep(time.Microsecond * 500)
			continue
		}
	}
	log.Printf("花费了 %v 秒启动\n✅ ClickHouse 连接成功\n", (0.5)*float64(i))
	createTableSQL := `CREATE TABLE IF NOT EXISTS shit.toilet_log
(
    id           UInt64,               -- 唯一ID
    log_time     DateTime,             -- 拉屎时间，精确到秒
    smoothness   Enum8(                -- 拉屎顺畅程度
                    'blocked' = 0,     -- 完全拉不出
                    'hardly'  = 1,     -- 几乎拉不出
                    'normal'  = 2,     -- 正常拉屎
                    'diarrhea'= 3      -- 一泻千里
                  )
)
ENGINE = MergeTree
ORDER BY log_time;`
	if err := clickhouseConn.Exec(context.Background(), createTableSQL); err != nil {
		log.Fatal("执行 SQL 失败:", err)
	}
	return nil
}

// 检查端口监听，重试机制
func waitForPortAvailable(port int, maxRetries int, retryInterval time.Duration) bool {
	address := fmt.Sprintf(":%d", port)
	for i := 1; i <= maxRetries; i++ {
		// 尝试监听
		ln, err := net.Listen("tcp", address)
		if err == nil {
			ln.Close()
			fmt.Printf("%v 业务未在线，等待 %v 后重试...\n", port, retryInterval)
			time.Sleep(retryInterval)
			continue
		}
		// 打印失败信息
		// fmt.Printf("第 %d 次监听端口 %d 失败: %s\n", i, port, err)
		return true
		// 如果没到最大次数，等待再试
		// if i < maxRetries {
		// 	fmt.Printf("等待 %v 后重试...\n", retryInterval)
		// 	time.Sleep(retryInterval)
		// }
	}
	// 超出最大次数，返回 false
	return false
}
