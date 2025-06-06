package common

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/alloydbconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	REGION       = os.Getenv("REGION")
	PROJECT_ID   = os.Getenv("PROJECT_ID")
	ADB_IAM_USER = os.Getenv("ADB_IAM_USER")
	ADB_CLUSTER  = os.Getenv("ADB_CLUSTER")
	ADB_INSTANCE = os.Getenv("ADB_INSTANCE")
	ADB_IP       = os.Getenv("ADB_IP")
	ADB_PORT     = os.Getenv("ADB_PORT")
	ADB_DATABASE = os.Getenv("ADB_DATABASE")

	ML_EMBEDDING_MODEL = os.Getenv("ML_EMBEDDING_MODEL")
	ML_GEN_AI_MODEL    = os.Getenv("ML_GEN_AI_MODEL")

	ML_MAX_OUTPUT_TOKENS = os.Getenv("ML_MAX_OUTPUT_TOKENS")
	ML_TOPK              = os.Getenv("ML_TOPK")
	ML_TOPP              = os.Getenv("ML_TOPP")
	ML_TEMPERATURE       = os.Getenv("ML_TEMPERATURE")
)

func SaveSumamry(ctx context.Context, resourceSummary *FHIRResourceSumamry) error {

	fmt.Println("Saving resource summary to AlloyDB")

	dialer, err := alloydbconn.NewDialer(ctx, alloydbconn.WithIAMAuthN())
	if err != nil {
		return fmt.Errorf("Failed to init Dialer: %v", err)
	}

	fmt.Println("Got the New Dialer")

	port, err := strconv.Atoi(ADB_PORT)
	if err != nil {
		return fmt.Errorf("Invalid Port: %v", err)
	}

	//get the accesstoken for the default cloud fucntion service accont: {PROJECT-ID}@appspot.gserviceaccount.com
	accessToken, err := GetAccessToken()
	if err != nil {
		return fmt.Errorf("failed to obtain access token: %v", err)
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=require",
		ADB_IP, port, ADB_IAM_USER, accessToken, ADB_DATABASE)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("Failed to parse pgx config: %v", err)
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
		return fmt.Errorf("Failed to Connect to AlloyDB: %v", err)
	}
	defer conn.Close()

	//insert the data to AlloyDB
	err = insertData(conn, ctx, resourceSummary)
	if err != nil {
		return fmt.Errorf("Failed to Insert into AlloyDB: %v", err)
	}
	fmt.Println("Saved resource summary to AlloyDB")

	return nil
}

func insertData(conn *pgxpool.Pool, ctx context.Context, resourceSummary *FHIRResourceSumamry) error {

	id := resourceSummary.ResourceId
	resourceType := resourceSummary.ResourceType
	patientId := resourceSummary.PatientId
	data := resourceSummary.OriginalFHIRJSON
	epochMicroseconds := int64(resourceSummary.Timestamp)
	epochSeconds := epochMicroseconds / 1e6
	timestamp := time.Unix(epochSeconds, 0)
	timestampStr := timestamp.Format("2006-01-02 15:04:05")
	prompt := getContentGenPrompt(data)

	// Prepare the SQL statement for inserting a row.
	// The ML_PREDICT_ROW function is used to call the Text Bison model and generate content for summary column
	stmt := `
		INSERT INTO public.resources (id, type, patientId, data,summary,timestamp) 
		VALUES ($1, $2, $3, $4,
			ML_PREDICT_ROW(
					'publishers/google/models/' || $7,  
				    json_build_object('instances', 
 						json_build_object('content', $6::text),
						'parameters', json_build_object('maxOutputTokens', $8::numeric,'topK', $9::numeric,'topP', $10::float,'temperature', $11::float)
					)
			)->'predictions'->0->>'content',
		    $5
		)
	`
	// Execute the SQL statement with the variable values
	_, err := conn.Exec(ctx, stmt, id, resourceType, patientId, data, timestampStr, prompt,
		ML_GEN_AI_MODEL, ML_MAX_OUTPUT_TOKENS, ML_TOPK, ML_TOPP, ML_TEMPERATURE)
	if err != nil {
		return fmt.Errorf("Unable to insert row into AlloyDB: %v", err)
	}
	fmt.Println("Row inserted successfully")

	return nil
}

func getContentGenPrompt(fhirJSONString string) string {
	return "You are a clinician and also an expert on Healthcare data especially FHIR JSON." +
		"And you are also very sensitive about patient privacy and protecting PII like their names," +
		"emails, phone numbers, addresses and date of birth. " +
		"I would like you to summarize the below FHIR JSON into a short paragraph that includes the following info:" +
		"- convert the timestamp to a string that shows how many years/months/days/hours ago is the data " +
		"- Any other relationships to other resources based on the references if there are and a " +
		"- A clinical narrative of what the resource data is all about." +
		"Include data values and units. " +
		"Do not include any Personally Identifiable Information like names, emails,phone , addresses and date of birth. " +
		"Make the final conent to be a paragraph of text so I can use that for" +
		"generating Emebeddings out of it " +
		"here is the JOSN:" + fhirJSONString + "" +
		"important:  Do not include any names of patient's and practitioners in the final content"
}
