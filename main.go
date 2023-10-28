package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/shirou/gopsutil/process"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Company struct {
	Id string `json:"id"`
	Name string `json:"name"`
	SchemaSuffix string `json:"schema_suffix"`
}

func main() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	OmnioDb, err := gorm.Open(postgres.Open(GenerateDsn(
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USERNAME"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_DATABASE"),
	)), &gorm.Config{})

	if err != nil {
		panic(err)
	}

	var companies []Company
	if error := OmnioDb.Find(&companies).Error; error != nil {
		panic(error)
	}

	var commands = []exec.Cmd{}

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"companies": companies,
			"queues": unsafe.Sizeof(&commands),
			"memory": m.Alloc,
		})
	})
	fmt.Println("running commands now")
	go runQueues(companies, &commands)
	r.Run(":8080")
}

func runQueues(companies []Company, commands *[]exec.Cmd) {
	//for each company run a command 'php artisan --queue=the_company_name'
	cmd := exec.Command("php", "artisan", "queue:work")
	*commands = append(*commands, *cmd)

	fmt.Println("running queue for default")
	go runDefaultQueue(&(*commands)[0])
	for index, company := range companies {
		fmt.Println("running queue for: ", company.SchemaSuffix)
		cmd := exec.Command("php", "artisan", "queue:work", "--queue", company.SchemaSuffix)
		*commands = append(*commands, *cmd)
		go runDefaultQueue(&(*commands)[index+1])
	}
}

func runDefaultQueue(cmd *exec.Cmd) {
	/* err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Wait()
	if err != nil {
		log.Fatal(err)
	}
	usage := cmd.ProcessState.SysUsage()
	fmt.Printf("Memory usage: %v\n", usage) */



	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		// Measure memory usage here
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		fmt.Printf("Memory usage: %v bytes\n", m.Alloc)
		// Calculate memory usage

		p, err := process.NewProcess(int32(cmd.Process.Pid))
		if err != nil {
			log.Fatal(err)
		}
		memInfo, err := p.MemoryInfo()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Memory Usage: %d bytes\n", memInfo.RSS)

		/* if memory, error := calculateMemory(cmd.Process.Pid); error != nil {
			fmt.Println(error)
		} else {
			fmt.Printf("Memory usage by PID: %v bytes\n", memory)
		} */
		// Break the loop if needed
		// if someCondition {
		//     break
		// }
	}
	err = cmd.Wait()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Command finished executing")

	/* output, err := cmd.CombinedOutput()

	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
	} */
}

func GenerateDsn(host, port, username, password, database string) string{
	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", username, password, host, port, database)
}

func calculateMemory(pid int) (uint64, error) {
	//file, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	file, err := os.Open(fmt.Sprintf("/proc/%d/smaps", pid))
	if err != nil {
		return 0, err
	}
	defer file.Close()

	res := uint64(0)
	pfx := [] byte("VmRSS:")
	r := bufio.NewScanner(file)
	for r.Scan() {
		line := r.Bytes()
		if bytes.HasPrefix(line, pfx) {
			var size uint64
			_, err := fmt.Sscanf(string(line[len(pfx):]), "%d", &size)
			if err != nil {
				return 0, err
			}
			res += size
		}
	}
	if err := r.Err(); err != nil {
		return 0, err
	}
	return res, nil
}
