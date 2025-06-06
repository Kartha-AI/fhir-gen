package sofhir

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"cloud.google.com/go/alloydbconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ExecuteRagFunction(ctx context.Context, patientId string, prompt string) (*RagFunctionResponse, error) {

	fmt.Println("Executing rag Function in  AlloyDB..")
	fmt.Println("PatientId: ", patientId)
	fmt.Println("Prompt: ", prompt)

	conn, err := getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("Unable to connect to AlloyDB: %v", err)
	}
	defer conn.Close()

	//run SQL to execure the function
	query := fmt.Sprintf("select * from rag('%s', '%s')", patientId, prompt)

	fmt.Println("Executing the Query: ", query)

	// Execute the query
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Check if there's a row
	if !rows.Next() {
		return nil, fmt.Errorf("No rows returned from the query")
	}

	// Initialize RagFunctionResponse
	var response RagFunctionResponse

	// Scan the entire JSON response into a single variable
	var jsonResult json.RawMessage
	if err := rows.Scan(&jsonResult); err != nil {
		return nil, fmt.Errorf("Failed to scan JSON result: %v", err)
	}

	// Unmarshal the raw JSON data into RagFunctionResponse
	if err := json.Unmarshal(jsonResult, &response); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal JSON result: %v", err)
	}

	fmt.Println("Executed rag Function in AlloyDB")

	return &response, nil
}

func getConnection(ctx context.Context) (*pgxpool.Pool, error) {

	fmt.Println("Getting the AlloyDB Connection with pgx pool..")

	dialer, err := alloydbconn.NewDialer(ctx, alloydbconn.WithIAMAuthN())
	if err != nil {
		return nil, fmt.Errorf("Failed to init Dialer: %v", err)
	}

	fmt.Println("Got the New Dialer")

	port, err := strconv.Atoi(ADB_PORT)
	if err != nil {
		return &pgxpool.Pool{}, fmt.Errorf("Invalid Port: %v", err)
	}

	//get the accesstoken for the default cloud fucntion service accont: {PROJECT-ID}@appspot.gserviceaccount.com
	accessToken, err := GetAccessToken(ctx)
	if err != nil {
		return &pgxpool.Pool{}, fmt.Errorf("failed to obtain access token: %v", err)
	}

	fmt.Println("Got the Access Token")

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=require",
		ADB_IP, port, ADB_IAM_USER, accessToken, ADB_DATABASE)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return &pgxpool.Pool{}, fmt.Errorf("Failed to parse pgx config: %v", err)
	}

	fmt.Println("Parsed the Config")

	// Tell the driver to use the AlloyDB Go Connector to create connections
	config.ConnConfig.DialFunc = func(ctx context.Context, _ string, instance string) (net.Conn, error) {
		dbInstance := fmt.Sprintf("projects/%s/locations/%s/clusters/%s/instances/%s",
			PROJECT_ID, REGION, ADB_CLUSTER, ADB_INSTANCE)
		return dialer.Dial(ctx, dbInstance)
	}
	fmt.Println("Defined the DialFunc")

	// Create a DSN string from the config
	dsnString := config.ConnString()
	fmt.Println("Created the DSN String")

	conn, err := pgxpool.New(ctx, dsnString)
	if err != nil {
		fmt.Println("Failed to get the connection")
		return &pgxpool.Pool{}, fmt.Errorf("Failed to Connect to AlloyDB: %v", err)
	}

	fmt.Println("Got the AlloyDB Connection")

	return conn, nil
}
