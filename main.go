package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"runtime"
	"strings"

	"github.com/alecthomas/kingpin"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

var (
	appgroup  string
	app       string
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()
)

var (
	paramsYAML = kingpin.Flag("params-yaml", "Extension parameters, created from custom properties.").Envar("ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES_YAML").Required().String()

	gitSource = kingpin.Flag("source", "Source of the repository.").Envar("ESTAFETTE_GIT_SOURCE").Required().String()
	gitOwner  = kingpin.Flag("owner", "Ownner of the repository.").Envar("ESTAFETTE_GIT_OWNER").Required().String()
	gitName   = kingpin.Flag("name", "Name of the repository.").Envar("ESTAFETTE_GIT_NAME").Required().String()

	buildID   = kingpin.Flag("build-id", "Build ID.").Envar("ESTAFETTE_BUILD_ID").String()
	releaseID = kingpin.Flag("release-id", "Release ID.").Envar("ESTAFETTE_RELEASE_ID").String()

	stageName   = kingpin.Flag("stage-name", "The name of the.").Envar("ESTAFETTE_STAGE_NAME").Required().String()
	releaseName = kingpin.Flag("release-name", "Name of the release section, which is used by convention to resolve the credentials.").Envar("ESTAFETTE_RELEASE_NAME").String()

	credentialsPath = kingpin.Flag("credentials-path", "Path to file with GKE credentials configured at service level, passed in to this trusted extension.").Default("/credentials/kubernetes_engine.json").String()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// init log format from envvar ESTAFETTE_LOG_FORMAT
	foundation.InitLoggingFromEnv(appgroup, app, version, branch, revision, buildDate)

	// create context to cancel commands on sigterm
	ctx := foundation.InitCancellationContext(context.Background())

	log.Info().Msg("Unmarshalling credentials parameter...")
	var params Params
	err := yaml.Unmarshal([]byte(*paramsYAML), &params)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed unmarshalling credential parameter")
	}

	log.Info().Msg("Setting default for credential parameter...")
	params.SetDefaults(*releaseName)

	log.Info().Msg("Unmarshalling injected credentials...")
	var credentials []GKECredentials
	// use mounted credential file if present instead of relying on an envvar
	if runtime.GOOS == "windows" {
		*credentialsPath = "C:" + *credentialsPath
	}
	if foundation.FileExists(*credentialsPath) {
		log.Info().Msgf("Reading credentials from file at path %v...", *credentialsPath)
		credentialsFileContent, err := ioutil.ReadFile(*credentialsPath)
		if err != nil {
			log.Fatal().Msgf("Failed reading credential file at path %v.", *credentialsPath)
		}
		err = json.Unmarshal(credentialsFileContent, &credentials)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed unmarshalling injected credentials")
		}
	} else {
		log.Fatal().Msg("Credentials of type kubernetes-engine are not injected; configure this extension as trusted and inject credentials of type kubernetes-engine")
	}

	log.Info().Msgf("Checking if credential %v exists...", params.Credentials)
	credential := GetCredentialsByName(credentials, params.Credentials)
	if credential == nil {
		log.Fatal().Err(err).Msgf("Credential with name %v does not exist.", params.Credentials)
	}

	log.Info().Msg("Unmarshalling parameters / custom properties...")
	if credential.AdditionalProperties.Defaults != nil {
		log.Info().Msgf("Using defaults from credential %v...", params.Credentials)
		params = *credential.AdditionalProperties.Defaults
	}
	err = yaml.Unmarshal([]byte(*paramsYAML), &params)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed unmarshalling parameters")
	}

	log.Info().Msg("Retrieving service account email from credentials...")
	var keyFileMap map[string]interface{}
	err = json.Unmarshal([]byte(credential.AdditionalProperties.ServiceAccountKeyfile), &keyFileMap)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed unmarshalling service account keyfile")
	}
	var saClientEmail string
	if saClientEmailIntfc, ok := keyFileMap["client_email"]; !ok {
		log.Fatal().Err(err).Msg("Field client_email missing from service account keyfile")
	} else {
		if t, aok := saClientEmailIntfc.(string); !aok {
			log.Fatal().Err(err).Msg("Field client_email not of type string")
		} else {
			saClientEmail = t
		}
	}

	log.Info().Msgf("Storing gke credential %v on disk...", params.Credentials)
	err = ioutil.WriteFile("/key-file.json", []byte(credential.AdditionalProperties.ServiceAccountKeyfile), 0600)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed writing service account keyfile")
	}

	log.Info().Msg("Authenticating to google cloud")
	foundation.RunCommandWithArgs(ctx, "gcloud", []string{"auth", "activate-service-account", saClientEmail, "--key-file", "/key-file.json"})

	log.Info().Msgf("Setting gcloud account to %v", saClientEmail)
	foundation.RunCommandWithArgs(ctx, "gcloud", []string{"config", "set", "account", saClientEmail})

	log.Info().Msg("Setting gcloud project")
	foundation.RunCommandWithArgs(ctx, "gcloud", []string{"config", "set", "project", credential.AdditionalProperties.Project})

	log.Info().Msgf("Getting gke credentials for cluster %v", credential.AdditionalProperties.Cluster)
	clustersGetCredentialsArsgs := []string{"container", "clusters", "get-credentials", credential.AdditionalProperties.Cluster}
	if credential.AdditionalProperties.Zone != "" {
		clustersGetCredentialsArsgs = append(clustersGetCredentialsArsgs, "--zone", credential.AdditionalProperties.Zone)
	} else if credential.AdditionalProperties.Region != "" {
		clustersGetCredentialsArsgs = append(clustersGetCredentialsArsgs, "--region", credential.AdditionalProperties.Region)
	} else {
		log.Fatal().Err(err).Msg("Credentials have no zone or region; at least one of them has to be defined")
	}
	foundation.RunCommandWithArgs(ctx, "gcloud", clustersGetCredentialsArsgs)

	log.Info().Msgf("Running image %v in GKE cluster %v...", params.Remote.ContainerImage, credential.AdditionalProperties.Cluster)

	jobName := getJobName()
	args := []string{"run", jobName, "--rm=true", "--restart=Never", "-i", fmt.Sprintf("--image=%v", params.Remote.ContainerImage), "-n", params.Namespace}
	for k, v := range params.Remote.EnvVars {
		args = append(args, "--env", fmt.Sprintf("%v=%v", k, v))
	}
	if len(params.Remote.Commands) > 0 {
		cmdSeparator := ";"
		cmdStopOnErrorFlag := "set -e; "
		combinedCommands := cmdStopOnErrorFlag + strings.Join(params.Remote.Commands, cmdSeparator)

		args = append(args, "--command", "--", params.Remote.Shell, "-c", combinedCommands)
	}
	foundation.RunCommandWithArgs(ctx, "kubectl", args)
}

func getJobName() string {

	// create job name of max 63 chars
	maxJobNameLength := 63

	jobType := "stage"
	//jobType := "build"
	id := *buildID
	if *releaseID != "" {
		//jobType = "release"
		id = *releaseID
	}

	re := regexp.MustCompile("[^a-zA-Z0-9]+")
	repoName := re.ReplaceAllString(fmt.Sprintf("%v/%v", *gitOwner, *gitName), "-")
	stageNameCleaned := re.ReplaceAllString(*stageName, "-")

	maxRepoNameLength := maxJobNameLength - len(jobType) - 1 - len(id) - 1 - len(stageNameCleaned) - 1
	if len(repoName) > maxRepoNameLength {
		repoName = repoName[:maxRepoNameLength]
	}

	return strings.ToLower(fmt.Sprintf("%v-%v-%v-%v", jobType, stageNameCleaned, repoName, id))
}
