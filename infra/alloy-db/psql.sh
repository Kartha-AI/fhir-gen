#!/bin/bash

exec > /tmp/entrypoint.log 2>&1

# Update package lists
apt-get update
# Install PostgreSQL client
apt-get install -y postgresql-client

# Fetch metadata values
ALLOYDB_HOST=$(curl -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ALLOYDB_HOST")
ALLOYDB_USER=$(curl -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ALLOYDB_USER")
ALLOYDB_PASSWORD=$(curl -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ALLOYDB_PASSWORD")
ALLOYDB_DB=$(curl -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ALLOYDB_DB")
ALLOYDB_IAM_USER=$(curl -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ALLOYDB_IAM_USER")
ML_EMBEDDING_MODEL=$(curl -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ML_EMBEDDING_MODEL")
ML_GEN_AI_MODEL=$(curl -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ML_GEN_AI_MODEL")
ML_MAX_OUTPUT_TOKENS=$(curl -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ML_MAX_OUTPUT_TOKENS")
ML_TOPK=$(curl -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ML_TOPK")
ML_TOPP=$(curl -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ML_TOPP")
ML_TEMPERATURE=$(curl -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/ML_TEMPERATURE")

# Ensure you have the necessary error handling in case any metadata is not found
if [ -z "$ALLOYDB_HOST" ]; then
  echo "ALLOYDB_HOST not found in metadata"
  exit 1
fi

if [ -z "$ALLOYDB_USER" ]; then
  echo "ALLOYDB_USER not found in metadata"
  exit 1
fi

if [ -z "$ALLOYDB_PASSWORD" ]; then
  echo "ALLOYDB_PASSWORD not found in metadata"
  exit 1
fi

if [ -z "$ALLOYDB_DB" ]; then
  echo "ALLOYDB_DB not found in metadata"
  exit 1
fi

# Use the ALLOYDB_PASS environment variable for the session
export PGPASSWORD="$ALLOYDB_PASSWORD"

# Function to check if the database exists
database_exists() {
  psql -h "$ALLOYDB_HOST" -U "$ALLOYDB_USER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$ALLOYDB_DB'" | grep -q 1
  return $?
}

# Function to wait for database to be ready
wait_for_database() {
  for i in {1..10}; do # Wait up to 10 seconds
    if psql -h "$ALLOYDB_HOST" -U "$ALLOYDB_USER" -d "$ALLOYDB_DB" -c '\q' 2>/dev/null; then
      echo "Database $ALLOYDB_DB is ready."
      return 0
    else
      echo "Waiting for database $ALLOYDB_DB to be ready..."
      sleep 1
    fi
  done
  echo "Timed out waiting for database $ALLOYDB_DB."
  return 1
}

# Check if the database exists
if database_exists; then
  echo "Database $ALLOYDB_DB already exists."
else
  echo "Database $ALLOYDB_DB deosn't exist. creating new ... "
  # Attempt to create the database
  if psql -h "$ALLOYDB_HOST" -U "$ALLOYDB_USER" -d postgres -c "CREATE DATABASE $ALLOYDB_DB"; then

    echo "Successfully created database $ALLOYDB_DB."
  else
    echo "Failed to create database $ALLOYDB_DB."
    exit 1
  fi
fi

# Wait for the database to be ready
if ! wait_for_database; then
  exit 1
fi

# Create extensions if the database is ready
if ! psql "host=$ALLOYDB_HOST user=$ALLOYDB_USER dbname=$ALLOYDB_DB" -c "CREATE EXTENSION IF NOT EXISTS google_ml_integration CASCADE"; then
  echo "Failed to create google_ml_integration extension."
  exit 1
fi

if ! psql "host=$ALLOYDB_HOST user=$ALLOYDB_USER dbname=$ALLOYDB_DB" -c "CREATE EXTENSION IF NOT EXISTS vector"; then
  echo "Failed to create vector extension."
  exit 1
fi

echo "Extensions are in place."

# Create temporary file with SQL statements for creating the table  
temp_file1=$(mktemp)
cat <<EOF > "$temp_file1"
DO \$\$
BEGIN
    IF NOT EXISTS (
        SELECT 1 
        FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_name = 'resources'
    ) THEN
        -- Create the table
        CREATE TABLE public.resources (
            id VARCHAR(255) PRIMARY KEY,
            type VARCHAR(255) NOT NULL, 
            patientId VARCHAR(255) NOT NULL,
            timestamp TIMESTAMP NOT NULL, 
            summary TEXT,
            embedding VECTOR,
            data JSONB
        );
        GRANT SELECT, INSERT, UPDATE ON public.resources TO "$ALLOYDB_IAM_USER";
    END IF;
END \$\$;
EOF

cat "$temp_file1"

# execute psql from temp file
if ! psql -h "$ALLOYDB_HOST" -U "$ALLOYDB_USER" -d "$ALLOYDB_DB" -f "$temp_file1"; then
  echo "Failed to execute SQL commands from file to create the table."
  exit 1
else
    echo "Executed SQL successfuly and created tables in database $ALLOYDB_DB."
fi

# Create temporary file with SQL statements for creating the insert trigger for the loader
temp_file2=$(mktemp)
cat <<EOF > "$temp_file2"
CREATE OR REPLACE FUNCTION insert_resource_create_embedding()
	RETURNS trigger
	LANGUAGE 'plpgsql'
AS \$\$
	BEGIN
		SELECT embedding('$ML_EMBEDDING_MODEL', NEW.summary)
		INTO NEW.embedding;

		RETURN NEW;
	END \$\$;
	
CREATE TRIGGER insert_resource_trigger
	BEFORE INSERT
	ON public.resources
	FOR EACH ROW
     EXECUTE PROCEDURE insert_resource_create_embedding();
EOF

cat "$temp_file2"
# execute psql from temp file
if ! psql -h "$ALLOYDB_HOST" -U "$ALLOYDB_USER" -d "$ALLOYDB_DB" -f "$temp_file2"; then
  echo "Failed to execute SQL commands for creating the  insert trigger for the loader"
  exit 1
else
    echo "Executed SQL successfuly. Trigegrs are in place in database $ALLOYDB_DB."
fi

# Create temporary file with SQL statements for creating the rag function
temp_file3=$(mktemp)
cat <<EOF > "$temp_file3"
CREATE OR REPLACE FUNCTION rag(patient_id VARCHAR, input_prompt VARCHAR ) RETURNS TABLE (
    response json
)
AS \$\$
BEGIN
    RETURN QUERY
        WITH search_results as (
          SELECT string_agg(summary, '|') AS retrieved_summaries
          FROM (
            SELECT
              summary
            FROM
              public.resources r
            WHERE 
              patientid = patient_id
            ORDER BY
              (r.embedding <=> embedding('$ML_EMBEDDING_MODEL',input_prompt)::vector) ASC
            LIMIT 50
          ) AS subquery_alias
        ),

        prompt as (
          select
              'you are a clinician that can udnerstand patient electronic records and be able to answer users questions. 
            Based on the patient request we have retrieved a list of records closely related to users prompt. 
            The retrieved list is a pipe de-limited text of summaries derived from FHIR resources associated to the patient.Important note: hide any PII incvluding, Names, DOB, Address, Email and Phone numbers of Patients from the answer.
            Here is the list of summaries from the search:' || retrieved_summaries || '. And the question is: ' || input_prompt
            as prompt_text
          from
              search_results
        )
    
        select
          ml_predict_row(
            FORMAT('publishers/google/models/%s','$ML_GEN_AI_MODEL'),
            json_build_object(
              'instances',json_build_object('prompt',prompt_text),
              'parameters',json_build_object('maxOutputTokens',$ML_MAX_OUTPUT_TOKENS,'topK',$ML_TOPK,'topP',$ML_TOPP, 'temperature',$ML_TEMPERATURE)
            )
          ) as response
        from
            prompt;
END;
\$\$ LANGUAGE plpgsql;
EOF

cat "$temp_file3"
# execute psql from temp file
if ! psql -h "$ALLOYDB_HOST" -U "$ALLOYDB_USER" -d "$ALLOYDB_DB" -f "$temp_file3"; then
  echo "Failed to execute SQL commands for creating the RAG function"
  exit 1
else
    echo "Executed SQL successfuly and the RAG function is in place in database $ALLOYDB_DB."
fi


# Clean up environment variable after use and exit
unset PGPASSWORD
echo "Script completed successfully."
exit 0
