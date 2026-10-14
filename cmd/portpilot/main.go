package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"portpilot/internal/config"
	"portpilot/internal/ports"
	"portpilot/internal/process"
	"portpilot/internal/report"
)

func printUsage() {
	fmt.Println("PortPilot — Cross-Platform Port Inspector")
	fmt.Println("\nUsage:")
	fmt.Println("  portpilot check <port> [--timeout <duration>] [--config <file>]")
	fmt.Println("  portpilot inspect <port> [--timeout <duration>] [--config <file>]")
	fmt.Println("  portpilot scan --start <port> --end <port> [--timeout <duration>] [--format json] [--output <file>] [--config <file>]")
	fmt.Println("\nExamples:")
	fmt.Println("  portpilot check 3000")
	fmt.Println("  portpilot inspect 8080")
	fmt.Println("  portpilot scan --start 3000 --end 3100 --config .portpilot.json")
	fmt.Println("  portpilot list [--config <file>]")
	fmt.Println("  portpilot kill <port> [--force] [--timeout <duration>] [--config <file>]")
	fmt.Println("  portpilot watch <port> [--interval <duration>] [--timeout <duration>] [--config <file>]")
}

func loadCustomServices(configPath string) {
	if configPath == "" {
		if _, err := os.Stat(config.DefaultConfigFileName); err == nil {
			configPath = config.DefaultConfigFileName
		} else {
			return
		}
	}

	mappings, err := config.LoadConfig(configPath)
	if err == nil {
		for port, service := range mappings {
			ports.KnownServices[port] = service
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "check":
		checkCmd := flag.NewFlagSet("check", flag.ExitOnError)
		timeoutFlag := checkCmd.Duration("timeout", 2*time.Second, "Connection timeout duration")
		formatFlag := checkCmd.String("format", "text", "Output format (text or json)")
		configFlag := checkCmd.String("config", "", "Custom configuration file path")

		if err := checkCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
			os.Exit(1)
		}

		loadCustomServices(*configFlag)

		if *formatFlag != "text" && *formatFlag != "json" {
			fmt.Fprintf(os.Stderr, "Error: invalid format '%s' (allowed: text, json)\n", *formatFlag)
			os.Exit(1)
		}

		args := checkCmd.Args()
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "Error: missing port argument.")
			printUsage()
			os.Exit(1)
		}

		var port int
		_, err := fmt.Sscanf(args[0], "%d", &port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid port number '%s'\n", args[0])
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
		defer cancel()

		result := ports.CheckPort(ctx, port, *timeoutFlag)
		service := ports.GetKnownService(port)

		if *formatFlag == "json" {
			type CheckDTO struct {
				Port    int    `json:"port"`
				Status  string `json:"status"`
				Service string `json:"service,omitempty"`
				Message string `json:"message"`
				Error   string `json:"error,omitempty"`
			}

			errStr := ""
			if result.Error != nil {
				errStr = result.Error.Error()
			}

			dto := CheckDTO{
				Port:    result.Port,
				Status:  string(result.Status),
				Service: service,
				Message: result.Message,
				Error:   errStr,
			}

			data, _ := json.MarshalIndent(dto, "", "  ")
			fmt.Println(string(data))
			if result.Status == ports.StatusOccupied {
				os.Exit(1)
			}
			return
		}

		// Text output
		fmt.Printf("Result for port %d:\n", result.Port)
		fmt.Printf("  Status:  %s\n", result.Status)
		if service != "" {
			fmt.Printf("  Service: %s\n", service)
		}
		fmt.Printf("  Details: %s\n", result.Message)
		if result.Error != nil {
			fmt.Printf("  Error:   %v\n", result.Error)
		}

		if result.Status == ports.StatusOccupied {
			os.Exit(1)
		}

	case "inspect":
		inspectCmd := flag.NewFlagSet("inspect", flag.ExitOnError)
		timeoutFlag := inspectCmd.Duration("timeout", 2*time.Second, "Inspection timeout duration")
		formatFlag := inspectCmd.String("format", "text", "Output format (text or json)")
		configFlag := inspectCmd.String("config", "", "Custom configuration file path")

		if err := inspectCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
			os.Exit(1)
		}

		loadCustomServices(*configFlag)

		if *formatFlag != "text" && *formatFlag != "json" {
			fmt.Fprintf(os.Stderr, "Error: invalid format '%s' (allowed: text, json)\n", *formatFlag)
			os.Exit(1)
		}

		args := inspectCmd.Args()
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "Error: missing port argument.")
			printUsage()
			os.Exit(1)
		}

		var port int
		_, err := fmt.Sscanf(args[0], "%d", &port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid port number '%s'\n", args[0])
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
		defer cancel()

		portRes := ports.CheckPort(ctx, port, *timeoutFlag)
		service := ports.GetKnownService(port)

		var procInfo *process.Info
		if portRes.Status == ports.StatusOccupied {
			if p, err := process.FindProcessByPort(ctx, port); err == nil {
				procInfo = &p
			}
		}

		if *formatFlag == "json" {
			type InspectDTO struct {
				Port    int           `json:"port"`
				Status  string        `json:"status"`
				Service string        `json:"service,omitempty"`
				Message string        `json:"message"`
				Process *process.Info `json:"process,omitempty"`
			}

			dto := InspectDTO{
				Port:    port,
				Status:  string(portRes.Status),
				Service: service,
				Message: portRes.Message,
				Process: procInfo,
			}

			data, _ := json.MarshalIndent(dto, "", "  ")
			fmt.Println(string(data))
			return
		}

		// Text output
		fmt.Printf("Port Status: %s (%s)\n", portRes.Status, portRes.Message)
		if service != "" {
			fmt.Printf("Service Mapping: %s\n", service)
		}

		if portRes.Status == ports.StatusOccupied && procInfo != nil {
			fmt.Printf("Process Info:\n")
			fmt.Printf("  Verified: %v\n", procInfo.Verified)
			if procInfo.Verified {
				fmt.Printf("  Process:  %s\n", procInfo.Name)
				fmt.Printf("  PID:      %d\n", procInfo.PID)
			}
			fmt.Printf("  Details:  %s\n", procInfo.Message)
		}

	case "scan":
		scanCmd := flag.NewFlagSet("scan", flag.ExitOnError)
		startFlag := scanCmd.Int("start", 3000, "Start port")
		endFlag := scanCmd.Int("end", 3100, "End port")
		timeoutFlag := scanCmd.Duration("timeout", 1*time.Second, "Connection timeout duration")
		formatFlag := scanCmd.String("format", "text", "Output format (text or json)")
		outputFlag := scanCmd.String("output", "", "Output report file path (.json or .md)")
		configFlag := scanCmd.String("config", "", "Custom configuration file path")

		if err := scanCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
			os.Exit(1)
		}

		loadCustomServices(*configFlag)

		if *startFlag < 1 || *startFlag > 65535 || *endFlag < 1 || *endFlag > 65535 {
			fmt.Fprintln(os.Stderr, "Error: port numbers must be between 1 and 65535.")
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if *formatFlag != "text" && *formatFlag != "json" {
			fmt.Fprintf(os.Stderr, "Error: invalid format '%s' (allowed: text, json)\n", *formatFlag)
			os.Exit(1)
		}

		if *formatFlag == "text" && *outputFlag == "" {
			fmt.Printf("Scanning port range %d to %d...\n\n", *startFlag, *endFlag)
		}

		results := ports.ScanRange(ctx, *startFlag, *endFlag, *timeoutFlag)

		sort.Slice(results, func(i, j int) bool {
			return results[i].Port < results[j].Port
		})

		var items []report.Item
		occupiedCount := 0

		for _, r := range results {
			if r.Status == ports.StatusOccupied {
				occupiedCount++
			}

			service := ports.GetKnownService(r.Port)
			var procInfo *process.Info
			if r.Status == ports.StatusOccupied {
				if p, err := process.FindProcessByPort(ctx, r.Port); err == nil {
					procInfo = &p
				}
			}

			var errStr string
			if r.Error != nil {
				errStr = r.Error.Error()
			}

			items = append(items, report.Item{
				Port:        r.Port,
				Status:      r.Status,
				Service:     service,
				Message:     r.Message,
				Process:     procInfo,
				ErrorString: errStr,
			})
		}

		rep := report.Report{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			TotalPorts:  len(results),
			Occupied:    occupiedCount,
			Items:       items,
		}

		if *outputFlag != "" {
			var fileContent string
			var exportErr error

			if strings.HasSuffix(*outputFlag, ".json") {
				fileContent, exportErr = rep.ToJSON()
			} else {
				fileContent = rep.ToMarkdown()
			}

			if exportErr == nil {
				exportErr = report.SaveToFile(*outputFlag, fileContent)
			}

			if exportErr != nil {
				fmt.Fprintf(os.Stderr, "Error saving report to %s: %v\n", *outputFlag, exportErr)
				os.Exit(1)
			}
			fmt.Printf("Report successfully exported to %s\n", *outputFlag)
			return
		}

		if *formatFlag == "json" {
			jsonOut, err := rep.ToJSON()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(jsonOut)
			return
		}

		for _, item := range rep.Items {
			if item.Status == ports.StatusOccupied {
				hint := ""
				if item.Service != "" {
					hint = fmt.Sprintf(" [%s]", item.Service)
				}
				procDetails := ""
				if item.Process != nil && item.Process.Verified {
					procDetails = fmt.Sprintf(" (Process: %s, PID: %d)", item.Process.Name, item.Process.PID)
				}
				fmt.Printf("  [OCCUPIED] Port %d%s - %s%s\n", item.Port, hint, item.Message, procDetails)
			}
		}

		fmt.Printf("\nScan complete. Found %d occupied port(s) between %d and %d.\n", occupiedCount, *startFlag, *endFlag)

	case "list":
		listCmd := flag.NewFlagSet("list", flag.ExitOnError)
		configFlag := listCmd.String("config", "", "Custom configuration file path")
		formatFlag := listCmd.String("format", "text", "Output format (text or json)")

		if err := listCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
			os.Exit(1)
		}

		loadCustomServices(*configFlag)

		if *formatFlag != "text" && *formatFlag != "json" {
			fmt.Fprintf(os.Stderr, "Error: invalid format '%s' (allowed: text, json)\n", *formatFlag)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		activePorts, err := process.ListActivePorts(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing ports: %v\n", err)
			os.Exit(1)
		}

		// Sort by port number
		sort.Slice(activePorts, func(i, j int) bool {
			return activePorts[i].Port < activePorts[j].Port
		})

		// Attach service hints to active ports before rendering
		type ActivePortDTO struct {
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
			Service  string `json:"service,omitempty"`
			PID      int    `json:"pid,omitempty"`
			Process  string `json:"process,omitempty"`
			Verified bool   `json:"verified"`
		}

		var dtoList []ActivePortDTO
		for _, p := range activePorts {
			service := ports.GetKnownService(p.Port)
			dtoList = append(dtoList, ActivePortDTO{
				Port:     p.Port,
				Protocol: p.Protocol,
				Service:  service,
				PID:      p.PID,
				Process:  p.Name,
				Verified: p.Verified,
			})
		}

		if *formatFlag == "json" {
			type ListReport struct {
				GeneratedAt string          `json:"generated_at"`
				Total       int             `json:"total_active_ports"`
				Ports       []ActivePortDTO `json:"ports"`
			}

			rep := ListReport{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Total:       len(dtoList),
				Ports:       dtoList,
			}

			data, err := json.MarshalIndent(rep, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
			return
		}

		// Default text terminal table output
		fmt.Printf("Listing all active listening TCP ports...\n\n")
		fmt.Printf("%-8s  %-25s  %-15s  %s\n", "PORT", "SERVICE / HINT", "PID", "PROCESS")
		fmt.Println("------------------------------------------------------------------")

		for _, p := range dtoList {
			service := p.Service
			if service == "" {
				service = "-"
			}
			pidStr := "-"
			if p.PID > 0 {
				pidStr = fmt.Sprintf("%d", p.PID)
			}
			procName := p.Process
			if procName == "" {
				procName = "-"
			}

			fmt.Printf("%-8d  %-25s  %-15s  %s\n", p.Port, service, pidStr, procName)
		}

		fmt.Printf("\nTotal active listening ports found: %d\n", len(dtoList))

	case "kill":
		killCmd := flag.NewFlagSet("kill", flag.ExitOnError)
		timeoutFlag := killCmd.Duration("timeout", 3*time.Second, "Termination timeout duration")
		forceFlag := flag.Bool("force", false, "Skip confirmation prompt")
		configFlag := flag.String("config", "", "Custom configuration file path")

		if err := killCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
			os.Exit(1)
		}

		loadCustomServices(*configFlag)

		args := killCmd.Args()
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "Error: missing port argument.")
			printUsage()
			os.Exit(1)
		}

		var port int
		_, err := fmt.Sscanf(args[0], "%d", &port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid port number '%s'\n", args[0])
			os.Exit(1)
		}

		// Use a temporary context just for initial inspection/finding the process
		inspectCtx, inspectCancel := context.WithTimeout(context.Background(), 3*time.Second)
		procInfo, err := process.FindProcessByPort(inspectCtx, port)
		inspectCancel()

		if err != nil || !procInfo.Verified || procInfo.PID <= 0 {
			fmt.Fprintf(os.Stderr, "Error: No verified process found listening on port %d.\n", port)
			os.Exit(1)
		}

		fmt.Printf("Target Process Found:\n")
		fmt.Printf("  Port:     %d\n", port)
		fmt.Printf("  PID:      %d\n", procInfo.PID)
		fmt.Printf("  Process:  %s\n", procInfo.Name)
		fmt.Printf("  Details:  %s\n", procInfo.Message)

		// Explicit User Confirmation Prompt (unless --force is passed)
		if !*forceFlag {
			fmt.Printf("\nAre you sure you want to terminate process '%s' (PID: %d)? [y/N]: ", procInfo.Name, procInfo.PID)
			var response string
			_, _ = fmt.Scanln(&response)

			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Termination cancelled.")
				return
			}
		}

		// Now start the fresh context timer specifically for the termination command
		termCtx, termCancel := context.WithTimeout(context.Background(), *timeoutFlag)
		defer termCancel()

		fmt.Printf("Terminating process (PID: %d)...\n", procInfo.PID)
		if err := process.TerminateProcess(termCtx, procInfo.PID); err != nil {
			fmt.Fprintf(os.Stderr, "Error terminating process: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully terminated process %d on port %d.\n", procInfo.PID, port)

	case "watch":
		watchCmd := flag.NewFlagSet("watch", flag.ExitOnError)
		intervalFlag := watchCmd.Duration("interval", 2*time.Second, "Polling interval duration")
		timeoutFlag := watchCmd.Duration("timeout", 2*time.Second, "Check timeout duration")
		configFlag := watchCmd.String("config", "", "Custom configuration file path")

		if err := watchCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
			os.Exit(1)
		}

		loadCustomServices(*configFlag)

		args := watchCmd.Args()
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "Error: missing port argument.")
			printUsage()
			os.Exit(1)
		}

		var port int
		_, err := fmt.Sscanf(args[0], "%d", &port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid port number '%s'\n", args[0])
			os.Exit(1)
		}

		fmt.Printf("Watching port %d (polling every %v). Press Ctrl+C to exit.\n\n", port, *intervalFlag)

		var lastStatus ports.Status
		lastPID := 0

		// Setup clean exit on SIGINT (Ctrl+C)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)

		ticker := time.NewTicker(*intervalFlag)
		defer ticker.Stop()

		// Run an initial check immediately
		performWatchCheck(port, *timeoutFlag, &lastStatus, &lastPID)

		for {
			select {
			case <-sigChan:
				fmt.Println("\nStopped watching port.")
				return
			case <-ticker.C:
				performWatchCheck(port, *timeoutFlag, &lastStatus, &lastPID)
			}
		}

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: '%s'\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func performWatchCheck(port int, timeout time.Duration, lastStatus *ports.Status, lastPID *int) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res := ports.CheckPort(ctx, port, timeout)
	service := ports.GetKnownService(port)

	currentPID := 0
	procName := ""
	if res.Status == ports.StatusOccupied {
		if p, err := process.FindProcessByPort(ctx, port); err == nil && p.Verified {
			currentPID = p.PID
			procName = p.Name
		}
	}

	// Print state change or keep a heartbeat line
	timestamp := time.Now().Format("15:04:05")
	if res.Status != *lastStatus || currentPID != *lastPID {
		svcInfo := ""
		if service != "" {
			svcInfo = fmt.Sprintf(" (%s)", service)
		}
		procInfo := ""
		if currentPID > 0 {
			procInfo = fmt.Sprintf(" [PID: %d, Proc: %s]", currentPID, procName)
		}

		fmt.Printf("[%s] Status changed: %s%s%s\n", timestamp, res.Status, svcInfo, procInfo)
		*lastStatus = res.Status
		*lastPID = currentPID
	}
}
