package sofhir

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// Check if the user has the permissions to access requested data.
// It also returns the resoruceURI with appeneded patientId for patient role
// to restrict access to the resources that are related to the patient that user is associated with
func AuthorizeResourceAccess(
	ctx context.Context,
	claims map[string]interface{},
	requestType string,
	resourceType string,
	resourceURI string,
	requestBody []byte,
) (bool, string, error) {

	modifiedResourceURI := resourceURI
	role := claims[ROLE_CLAIM].(string)

	// make sure that the user with Patient Role is only accessing the data for the patient they are associated with
	if role == PATIENT_ROLE {
		userPatientId := claims[PATIENT_ID_CLAIM].(string)
		if requestType == GET_RERQUEST {
			return authorizePatientUserForReadAccess(userPatientId, resourceType, resourceURI)
		} else { //POST or PUT request
			return authorizePatientUserForWriteAccess(ctx, userPatientId, requestType, resourceType, resourceURI, requestBody)
		}
	} else if role == PROVIDER_ROLE {
		//Provide role is allwoed to access all patients data with in their organization
		//will return true, modifiedResourceURI, nil towards the end
	} else {
		return false, resourceURI, fmt.Errorf("Access Denied: Unknown Role: %s", role)
	}

	return true, modifiedResourceURI, nil
}

// check if the patient user with Patient Role is only accessing their own resources data
func authorizePatientUserForReadAccess(userPatientId string, resourceType string, resourceURI string) (bool, string, error) {
	if resourceType == PATIENT_RESOURCE_TYPE { //resourceURI begins with /Patient
		return authorizePatientUserForReadPatient(userPatientId, resourceType, resourceURI)
	} else { //All other resources
		return authorizePatientUserForReadResources(userPatientId, resourceType, resourceURI)
	}
}

// check if the patient user with Patient Role is only accessing their own resources data
func authorizePatientUserForReadPatient(userPatientId string, resourceType string, resourceURI string) (bool, string, error) {
	log.Print("Authorizing Patient User for Read Access on Patient Resource")
	modifiedResourceURI := resourceURI
	if strings.ContainsRune(resourceURI, '?') { //query
		//if the query parm contains _id then check if the patientId in the query is same as the user's patientId
		log.Printf("RsourceURI is /Patient?{query} or /Patient with no query: %s", resourceURI)
		if strings.Contains(resourceURI, ID_SEARCH_PARM) {
			log.Print("ResourceURI contains _id query parm")
			queryParams := getQueryParams(resourceURI)
			if patientId, ok := queryParams[ID_SEARCH_PARM]; ok {
				if patientId != userPatientId { //user requesting data for a different patient
					log.Printf("Access Denied: Patient Id on _id parm: %s is not the same as user's PatientId: %s", patientId, userPatientId)
					return false, resourceURI, fmt.Errorf("Access Denied: User can only access their Patient data")
				}
			}
		}
	} else { //get by Id
		pathParts := strings.Split(resourceURI, "/")
		if len(pathParts) > 2 { //resourceURI is /Patient/{id}
			patientId := pathParts[2]
			log.Printf("ResourceURI: %s. Patient Id: %s:", resourceURI, patientId)
			if patientId != userPatientId { //user requesting data for a different patient
				log.Printf("Access Denied: PatientId: %s is not the same as user's PatientId: %s", patientId, userPatientId)
				return false, resourceURI, fmt.Errorf("Access Denied: User can only access their Patient data")
			}
		}
	}

	//finally just to be sure: append patientId to the resourceURI to restrict user to access only their patient data
	modifiedResourceURI = appendPatientIdToResourceURI(resourceType, resourceURI, userPatientId)

	log.Printf("Access Granted and Modified ResourceURI: %s", modifiedResourceURI)
	return true, modifiedResourceURI, nil
}

func authorizePatientUserForReadResources(userPatientId string, resourceType string, resourceURI string) (bool, string, error) {
	//if the resourceURI has subject or patient as query parm:
	//then check if the patientId in the query is same as the user's patientId
	log.Print("Authorizing Patient User for Read Access on Other Resources")
	if strings.Contains(resourceURI, SUBJECT_SEARCH_PARM) || strings.Contains(resourceURI, PATIENT_ID_SEARCH_PARM) {
		log.Print("ResourceURI contains subject or patient query parm")
		queryParams := getQueryParams(resourceURI)
		if patientId, ok := queryParams[PATIENT_ID_SEARCH_PARM]; ok {
			if patientId != userPatientId { //user requesting data for a different patient
				log.Printf("Access Denied: Patient Id on patient parm: %s is not the same as user's PatientId: %s", patientId, userPatientId)
				return false, resourceURI, fmt.Errorf("Access Denied: User can only access their Patient data")
			}
		}
		if patientId, ok := queryParams[SUBJECT_SEARCH_PARM]; ok {
			if patientId != userPatientId { //user requesting data for a different patient
				log.Printf("Access Denied: Patient Id on subject parm: %s is not the same as user's PatientId: %s", patientId, userPatientId)
				return false, resourceURI, fmt.Errorf("Access Denied: User can only access their Patient data")
			}
		}
	}
	//append patientId to the resourceURI to restrict user to access only the resource data associated to thier patient
	modifiedResourceURI := appendPatientIdToResourceURI(resourceType, resourceURI, userPatientId)

	log.Printf("Access Granted and Modified ResourceURI: %s", modifiedResourceURI)
	return true, modifiedResourceURI, nil
}

// check if the patient user with Patient Role is only updating their own resources data
func authorizePatientUserForWriteAccess(
	ctx context.Context,
	userPatientId string,
	requestType string,
	resourceType string,
	resourceURI string,
	requestBody []byte) (bool, string, error) {

	if resourceType == PATIENT_RESOURCE_TYPE { //POST or PUT on Patient
		return authorizePatientUserForPatientWrite(userPatientId, requestType, resourceURI)
	} else { //POST ot PUT on other resourceTypes
		return authorizePatientUserForResourceWrite(ctx, userPatientId, requestType, resourceType, resourceURI, requestBody)
	}
}

// check if the patient user with Patient Role is only  writing(crate/update) thier own patient data
func authorizePatientUserForPatientWrite(userPatientId string, requestType string, resourceURI string) (bool, string, error) {
	log.Print("Atuhorizing Patient User for Write Access on Patient Resource")
	if requestType == POST_REQUEST { //POST request on Patient not allowed
		log.Print("Access Denied: Create is Not Allowed on Patient")
		return false, resourceURI, fmt.Errorf("Access Denied: Create is Not Allowed on Patient")
	}
	//for PUT request: Check if the resourceURI is for the Patient that user is associated with
	pathParts := strings.Split(resourceURI, "/")
	if len(pathParts) > 2 { //resourceURI is /Patient/{id}.
		patientId := pathParts[2]
		if patientId != userPatientId { //user posting or updating data for a different patient
			log.Printf("Access Denied: PatientId on path parm: %s is not the same as user's PatientId: %s", patientId, userPatientId)
			return false, resourceURI, fmt.Errorf("Access Denied: User can only access their data")
		}
	}

	log.Print("Access Granted")
	return true, resourceURI, nil
}

// check if the patient user with Patient Role is only writing(crate/update) their own resoruces
func authorizePatientUserForResourceWrite(
	ctx context.Context,
	userPatientId string,
	requestType string,
	resourceType string,
	resourceURI string,
	requestBody []byte) (bool, string, error) {
	log.Print("Authorizing Patient User for Write Access on Other Resources")
	if requestType == PUT_REQUEST { //PUT request on other resourceTypes
		resoruceId := strings.Split(resourceURI, "/")[2] //get the resoruceId on the resourceURI
		//check if the resource belongs to the patient that user is associated with
		log.Printf("Checking if %s/%s is associated with the user's patient:%s", resourceType, resoruceId, userPatientId)
		IsAssociated, err := IsResourceAssociatedWithPatient(ctx, userPatientId, resourceType, resoruceId)
		if err != nil {
			return false, resourceURI, fmt.Errorf("Access Denied: Error checking resource association: %v", err)
		}
		if !IsAssociated {
			log.Printf("Access Denied: %s/%s is not associated with the user's patient:%s", resourceType, resoruceId, userPatientId)
			return false, resourceURI, fmt.Errorf("Access Denied: User can only update their patient data")
		}
	}
	//check request body's subject attribute with patient reference same as the user's patientId
	log.Printf("Checking if the request body's subject attribute is associated with the user's patient:%s", userPatientId)
	var resource Resource
	if err := json.Unmarshal(requestBody, &resource); err != nil {
		log.Print("Access Denied: Error parsing request body")
		return false, resourceURI, fmt.Errorf("Access Denied: Error parsing request body:%v", err)
	}
	patientRef := resource.Subject.Reference //Patient/{id}
	patienRefId := strings.Split(patientRef, "/")[1]
	if patienRefId != userPatientId { //user posting or updating data for a different patient
		log.Printf("Access Denied: PatientId on subject attribute: %s is not the same as user's PatientId: %s", patientRef, userPatientId)
		return false, resourceURI, fmt.Errorf("Access Denied: User can only update data for their associated Patient")
	}

	return true, resourceURI, nil
}

func appendPatientIdToResourceURI(resourceType string, resourceURI string, patientId string) string {

	patientIdSearchParm := PATIENT_ID_SEARCH_PARM
	if resourceType == PATIENT_RESOURCE_TYPE {
		patientIdSearchParm = ID_SEARCH_PARM
	}

	if strings.ContainsRune(resourceURI, '?') {
		return resourceURI + "&" + patientIdSearchParm + "=" + patientId
	} else {
		return resourceURI + "?" + patientIdSearchParm + "=" + patientId
	}
}

// checks the requested access with the scopes in the user's claims
func CheckScopes(claims map[string]interface{}, resourceType string, requestType string) (bool, error) {
	role := claims["role"].(string)
	var requestScope string
	if requestType == http.MethodGet {
		requestScope = fmt.Sprintf("%s/%s.read", role, resourceType)
	} else {
		requestScope = fmt.Sprintf("%s/%s.write", role, resourceType)
	}
	log.Print("Requested Scope: " + requestScope)
	roleScopes := []string{}
	if role == PATIENT_ROLE {
		roleScopes = PATIENT_ROLE_SCOPES
	} else if role == PROVIDER_ROLE {
		roleScopes = PROVIDER_ROLE_SCOPES
	} else {
		log.Print("Invalid Role")
		return false, fmt.Errorf("Invalid Role")
	}
	log.Print("Role Scopes: " + strings.Join(roleScopes, ", "))

	if contains(roleScopes, requestScope) {
		log.Print("Scope Allowed: " + requestScope)
		return true, nil
	} else {
		log.Print("Scope Not allowed: " + requestScope)
		return false, fmt.Errorf("Scope Not Allowed: %s", requestScope)
	}
}

func contains(scopes []string, target string) bool {
	for _, scope := range scopes {
		if strings.Trim(scope, " ") == target {
			return true
		}
	}
	return false
}

func getQueryParams(resourceURI string) map[string]string {
	queryParams := make(map[string]string)
	queryParts := strings.Split(resourceURI, "?")
	if len(queryParts) > 1 {
		query := queryParts[1]
		pairs := strings.Split(query, "&")
		for _, pair := range pairs {
			kv := strings.Split(pair, "=")
			queryParams[kv[0]] = kv[1]
		}
	}
	return queryParams
}
