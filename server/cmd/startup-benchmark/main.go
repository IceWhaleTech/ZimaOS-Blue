package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type StartupPhase struct {
	name     string
	duration time.Duration
	memDelta int64
}

var phases []StartupPhase

func recordPhase(name string, startMem uint64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	endMem := m.Alloc

	duration := time.Since(time.Now())
	phases = append(phases, StartupPhase{
		name:     name,
		duration: duration,
		memDelta: int64((endMem - startMem) / 1024),
	})
}

func main() {
	fmt.Println("=== ZimaOS-Echo 详细启动性能分析 ===\n")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	totalStartMem := m.Alloc

	// 阶段 1: 配置加载
	fmt.Println("测试阶段 1: 配置加载")
	start := time.Now()
	runtime.ReadMemStats(&m)
	phaseMem := m.Alloc
	time.Sleep(1 * time.Millisecond) // 模拟配置加载
	phase1Duration := time.Since(start)
	runtime.ReadMemStats(&m)
	phase1Mem := (m.Alloc - phaseMem) / 1024
	fmt.Printf("  耗时: %.2fms, 内存增长: %d KB\n\n", phase1Duration.Seconds()*1000, phase1Mem)

	// 阶段 2: 日志初始化
	fmt.Println("测试阶段 2: 日志初始化")
	start = time.Now()
	runtime.ReadMemStats(&m)
	phaseMem = m.Alloc
	time.Sleep(2 * time.Millisecond)
	phase2Duration := time.Since(start)
	runtime.ReadMemStats(&m)
	phase2Mem := (m.Alloc - phaseMem) / 1024
	fmt.Printf("  耗时: %.2fms, 内存增长: %d KB\n\n", phase2Duration.Seconds()*1000, phase2Mem)

	// 阶段 3: 数据库初始化
	fmt.Println("测试阶段 3: 数据库初始化")
	start = time.Now()
	runtime.ReadMemStats(&m)
	phaseMem = m.Alloc

	dataDir := filepath.Join(os.Getenv("USERPROFILE"), ".zimaos-echo")
	os.MkdirAll(dataDir, 0755)
	dbPath := filepath.Join(dataDir, "test-benchmark.db")
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err == nil {
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(time.Hour)

		// 执行 PRAGMA 优化
		db.Exec("PRAGMA journal_mode=WAL")
		db.Exec("PRAGMA foreign_keys=ON")
		db.Close()
	}

	phase3Duration := time.Since(start)
	runtime.ReadMemStats(&m)
	phase3Mem := (m.Alloc - phaseMem) / 1024
	fmt.Printf("  耗时: %.2fms, 内存增长: %d KB\n\n", phase3Duration.Seconds()*1000, phase3Mem)

	// 阶段 4: 并行初始化服务 (模拟)
	fmt.Println("测试阶段 4: 并行初始化服务 (20个并发)")
	start = time.Now()
	runtime.ReadMemStats(&m)
	phaseMem = m.Alloc

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond) // 模拟服务初始化
		}()
	}
	wg.Wait()

	phase4Duration := time.Since(start)
	runtime.ReadMemStats(&m)
	phase4Mem := (m.Alloc - phaseMem) / 1024
	fmt.Printf("  耗时: %.2fms, 内存增长: %d KB\n\n", phase4Duration.Seconds()*1000, phase4Mem)

	// 阶段 5: 路由注册
	fmt.Println("测试阶段 5: 路由注册")
	start = time.Now()
	runtime.ReadMemStats(&m)
	phaseMem = m.Alloc
	time.Sleep(3 * time.Millisecond)
	phase5Duration := time.Since(start)
	runtime.ReadMemStats(&m)
	phase5Mem := (m.Alloc - phaseMem) / 1024
	fmt.Printf("  耗时: %.2fms, 内存增长: %d KB\n\n", phase5Duration.Seconds()*1000, phase5Mem)

	// 总结
	totalDuration := phase1Duration + phase2Duration + phase3Duration + phase4Duration + phase5Duration
	runtime.ReadMemStats(&m)
	totalEndMem := m.Alloc
	totalMemGrowth := (totalEndMem - totalStartMem) / 1024 / 1024

	fmt.Println("=== 性能总结 ===")
	fmt.Printf("配置加载:      %.2fms (%.1f%%)\n", phase1Duration.Seconds()*1000, float64(phase1Duration)/float64(totalDuration)*100)
	fmt.Printf("日志初始化:    %.2fms (%.1f%%)\n", phase2Duration.Seconds()*1000, float64(phase2Duration)/float64(totalDuration)*100)
	fmt.Printf("数据库初始化:  %.2fms (%.1f%%)\n", phase3Duration.Seconds()*1000, float64(phase3Duration)/float64(totalDuration)*100)
	fmt.Printf("并行服务初始化: %.2fms (%.1f%%)\n", phase4Duration.Seconds()*1000, float64(phase4Duration)/float64(totalDuration)*100)
	fmt.Printf("路由注册:      %.2fms (%.1f%%)\n", phase5Duration.Seconds()*1000, float64(phase5Duration)/float64(totalDuration)*100)
	fmt.Printf("\n总启动时间: %.2fms\n", totalDuration.Seconds()*1000)
	fmt.Printf("总内存增长: %.2f MB\n", float64(totalMemGrowth))
	fmt.Printf("Goroutine 数量: %d\n", runtime.NumGoroutine())

	fmt.Println("\n=== 性能优化建议 ===")
	if phase3Duration > phase1Duration*2 {
		fmt.Println("⚠️  数据库初始化耗时较长，建议:")
		fmt.Println("   - 使用连接池预热")
		fmt.Println("   - 延迟加载非关键数据库")
	}
	if phase4Duration > 50*time.Millisecond {
		fmt.Println("⚠️  并行服务初始化耗时较长，建议:")
		fmt.Println("   - 增加并发数量")
		fmt.Println("   - 优化单个服务初始化时间")
	}
}
