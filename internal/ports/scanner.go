package ports

import (
	"context"
	"sync"
	"time"
)

// KnownServices maps common development, database, caching, and infrastructure ports to their descriptions.
var KnownServices = map[int]string{
	// Frontend & Web Development
	3000: "Node.js Development Server",
	4173: "Vite Preview Server",
	4200: "Angular Development Server",
	5000: "Common Development Service / Flask",
	5173: "Vite Development Server",
	8000: "Common Development Server / Django / FastAPI",
	8080: "Common Web Server / Tomcat",

	// Databases & Caching
	1433:  "Microsoft SQL Server",
	1521:  "Oracle Database",
	3306:  "MySQL / MariaDB Database",
	5432:  "PostgreSQL Database",
	6379:  "Redis In-Memory Data Store",
	7000:  "Cassandra / ScyllaDB",
	9042:  "Cassandra Native Transport",
	11211: "Memcached Caching Server",
	27017: "MongoDB NoSQL Database",
	27018: "MongoDB Shard / Secondary",
	5433:  "PostgreSQL (Alternate)",
	6380:  "Redis (Alternate)",

	// Message Queues & Streaming
	5672: "RabbitMQ Message Broker",
	6651: "Apache Pulsar",
	9092: "Apache Kafka Broker",

	// Search & Analytics
	8983: "Apache Solr Search Engine",
	9200: "Elasticsearch HTTP API",
	9300: "Elasticsearch Transport Node",

	// IoT, Smart Home, Streaming & Cameras
	554:  "RTSP Streaming (Wi-Fi IP Cameras / Security Cams)",
	1400: "Sonos Smart Speaker System",
	3689: "Apple DAAP / Media Streaming",
	5353: "mDNS / Bonjour (Smart Home Discovery)",
	8008: "Google Chromecast / Cast Device",
	8009: "Chromecast DIAL Protocol",
	8081: "Smart Home Hub / Alternate HTTP",

	// others
	8085: "Home Assistant / Smart Home Hub",
	8086: "InfluxDB Time Series Database",
	9000: "SonarQube / Code Quality Analysis",
	9090: "Prometheus Monitoring / Metrics",
	5939: "TeamViewer Remote Access",

	// PortPilot & Development Tools
	34115: "PortPilot CLI / API (Local HTTP)",
}

// GetKnownService returns the description for a port if it matches a well-known convention.
func GetKnownService(port int) string {
	if desc, ok := KnownServices[port]; ok {
		return desc
	}
	return ""
}

// ScanRange inspects a range of ports concurrently with bounded workers.
func ScanRange(ctx context.Context, start, end int, timeout time.Duration) []Result {
	if start > end {
		start, end = end, start
	}

	totalPorts := (end - start) + 1
	resultsChan := make(chan Result, totalPorts)

	// Limit concurrency to 50 concurrent workers to avoid OS socket exhaustion
	const maxWorkers = 50
	sem := make(chan struct{}, maxWorkers)

	var wg sync.WaitGroup

	for p := start; p <= end; p++ {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()

			// Acquire semaphore slot or respect context cancellation
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				resultsChan <- Result{
					Port:    port,
					Status:  StatusError,
					Message: "Scan canceled or timed out",
					Error:   ctx.Err(),
				}
				return
			}

			res := CheckPort(ctx, port, timeout)
			resultsChan <- res
		}(p)
	}

	// Close results channel once all workers finish
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	results := make([]Result, 0, totalPorts)
	for res := range resultsChan {
		results = append(results, res)
	}

	return results
}
