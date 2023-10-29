package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Company struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	SchemaSuffix string `json:"schema_suffix"`
}

type CompanyWithCommand struct {
	Company Company
	Command exec.Cmd
}

type CompanyWithRamUsage struct {
	Name     string `json:"name"`
	RamUsage uint64 `json:"ram_usage"`
}

type CompanyCmd map[string]struct {
	Company Company
	Cmd     *exec.Cmd
}

func main() {
	if err := godotenv.Load(); err != nil {
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

	//make a structure where the key is a string and values a couple of key called 'company' for company structure and 'cmd' for exec.cmd structure
	var companyCmds = make(CompanyCmd)

	r := gin.Default()
	r.GET("/info", func(c *gin.Context) {
		v, _ := mem.VirtualMemory()
		c.JSON(http.StatusOK, gin.H{
			"companies": GenerateResponse(&companyCmds),
			"total":     v.Total,
			"free":      v.Free,
			"used":      v.Used,
		})
	})
	fmt.Println("running commands now")
	//go runQueues(companies, &commands)
	GenerateCompanyCmd(&companyCmds, companies)
	//go runQueues(&companyCmds)
	r.Run(":8080")
}

func GenerateCompanyCmd(compCmd *CompanyCmd, companies []Company) {
	fmt.Println("generaring Company Cmd")
	var newcompCmd = make(CompanyCmd)
	for _, company := range companies {
		newcompCmd[company.SchemaSuffix] = struct {
			Company Company
			Cmd     *exec.Cmd
		}{company, runCommand([]string{"php", "artisan", "queue:work", "--queue", company.SchemaSuffix})}
	}
	*compCmd = newcompCmd
}

func GenerateResponse(companyCmds *CompanyCmd) []CompanyWithRamUsage {
	companiesWithHisRamUsage := []CompanyWithRamUsage{}
	//for each all make
	for _, companyCmd := range *companyCmds {
		fmt.Println("company is: ", &companyCmd.Company.Name)
		fmt.Println("cmd id: ", companyCmd.Cmd)
		fmt.Println("cmd process: ", companyCmd.Cmd.Process)
		fmt.Println("cmd PID: ", companyCmd.Cmd.Process.Pid)
		var companyWithHisRamUsage = CompanyWithRamUsage{}
		companyWithHisRamUsage.Name = companyCmd.Company.Name
		process, err := process.NewProcess(int32(companyCmd.Cmd.Process.Pid))
		if err != nil {
			log.Fatal(err)
		}
		processInfo, err := process.MemoryInfo()
		if err != nil {
			log.Fatal(err)
		}
		companyWithHisRamUsage.RamUsage = uint64(processInfo.RSS)
		companiesWithHisRamUsage = append(companiesWithHisRamUsage, companyWithHisRamUsage)
	}
	return companiesWithHisRamUsage
}

func runCommand(command []string) *exec.Cmd {
	cmd := exec.Command(command[0], command[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
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
		}
		err = cmd.Wait()
		if err != nil {
			log.Fatal(err)
		}
	}()
	fmt.Println("Command finished executing")
	return cmd
}

func GenerateDsn(host, port, username, password, database string) string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", username, password, host, port, database)
}
