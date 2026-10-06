// mypage-operator-dryrun is an emulator-only diagnostic. It contains no execute
// mode, ADC discovery, Google API, auth revocation, notification or write port.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"app.modules/core/mypage"
)

type configuration struct {
	project, environment, emulator string
	purpose                        mypage.SupportPurpose
}

func configurationFrom(get func(string) string, args []string) (configuration, error) {
	fail := func() (configuration, error) {
		return configuration{}, errors.New("emulator-only configuration unavailable")
	}
	if len(args) != 0 || get("GOOGLE_APPLICATION_CREDENTIALS") != "" || get("FIREBASE_AUTH_EMULATOR_HOST") != "" {
		return fail()
	}
	c := configuration{project: get("GOOGLE_CLOUD_PROJECT"), environment: get("MYPAGE_ENVIRONMENT"), emulator: get("FIRESTORE_EMULATOR_HOST"), purpose: mypage.SupportPurpose(get("MYPAGE_SUPPORT_PURPOSE"))}
	if c.environment != "development" || !strings.HasPrefix(c.project, "demo-") || len(c.project) > 30 || len(c.project) < 6 {
		return fail()
	}
	for _, ch := range c.project {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return fail()
		}
	}
	if c.purpose != mypage.SupportDelete && c.purpose != mypage.SupportRevoke && c.purpose != mypage.SupportDisclosure {
		return fail()
	}
	for _, key := range []string{"GCLOUD_PROJECT", "FIREBASE_PROJECT_ID"} {
		if value := get(key); value != "" && value != c.project {
			return fail()
		}
	}
	host, port, err := net.SplitHostPort(c.emulator)
	if err != nil || (host != "127.0.0.1" && host != "::1") {
		return fail()
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fail()
	}
	return c, nil
}

func readRequestRef(input io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(input, 66))
	if err != nil {
		return "", errors.New("receipt reference unavailable")
	}
	value := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if len(value) != 64 || len(data) > 65 {
		return "", errors.New("receipt reference unavailable")
	}
	for _, ch := range value {
		if (ch < 'a' || ch > 'f') && (ch < '0' || ch > '9') {
			return "", errors.New("receipt reference unavailable")
		}
	}
	return value, nil
}

func run(ctx context.Context, get func(string) string, args []string, input io.Reader, output io.Writer) error {
	config, err := configurationFrom(get, args)
	if err != nil {
		return err
	}
	ref, err := readRequestRef(input)
	if err != nil {
		return err
	}
	// The endpoint is explicitly fixed to an IP loopback emulator. Do not discover
	// credentials or allow SDK fallback to a live endpoint.
	client, err := firestore.NewClient(ctx, config.project, option.WithEndpoint(config.emulator), option.WithoutAuthentication(), option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	if err != nil {
		return errors.New("dryrun unavailable")
	}
	report, err := mypage.BuildDryRun(ctx, mypage.DryRunSelector{Environment: config.environment, ExpectedProject: config.project, RequestRef: ref, Purpose: config.purpose, Now: time.Now().UTC()}, &mypage.FirestoreDryRunReader{Client: client})
	closeErr := client.Close()
	if err != nil || closeErr != nil {
		return errors.New("dryrun unavailable")
	}
	if err := json.NewEncoder(output).Encode(report); err != nil {
		return errors.New("dryrun output unavailable")
	}
	return nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := run(ctx, os.Getenv, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "MyPage read-only emulator diagnostic unavailable")
		os.Exit(1)
	}
}
