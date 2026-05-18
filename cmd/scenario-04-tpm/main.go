package main

import (
	"path/filepath"
	"time"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/certs"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
)

func main() {
	scenario.Run(scenario.Config{
		Name:    "TPM",
		DirName: "04-tpm",
		Up:      up,
		Done:    done,
	})
}

func up(ctx *scenario.Context) error {
	serverCaPath := filepath.Join(ctx.ScenarioDir, "spire", "server", "devid-ca.pem")
	agentCertPath := filepath.Join(ctx.ScenarioDir, "spire", "agent", "devid-cert.pem")
	agentKeyPath := filepath.Join(ctx.ScenarioDir, "spire", "agent", "devid-key.pem")

	err := ctx.Runner.Run(
		"Provisioning x509pop Certificates",
		"In a real environment, a TPM holds a DevID certificate provisioned during\n"+
			"manufacturing. For this demo, we generate equivalent certificates:\n\n"+
			"  - A self-signed CA certificate (trusted by the SPIRE server)\n"+
			"  - An agent certificate signed by that CA (proves agent identity)\n"+
			"  - The agent's private key\n\n"+
			"The SPIRE server is configured to trust the CA, so any agent presenting\n"+
			"a certificate signed by this CA will be attested via x509pop.",
		func() error {
			if certs.FilesExist(serverCaPath, agentCertPath, agentKeyPath) {
				ctx.Log.Info("Certificates already exist, skipping provisioning.")
				return nil
			}

			ctx.Log.Info("Generating CA certificate...")
			ca, err := certs.GenerateCA("Demo DevID CA")
			if err != nil {
				return err
			}

			ctx.Log.Info("Generating agent certificate...")
			agent, err := certs.GenerateAgentCert(ca, "SPIRE Agent DevID")
			if err != nil {
				return err
			}

			if err := certs.WriteFile(serverCaPath, ca.CertPEM, ctx.Log); err != nil {
				return err
			}
			if err := certs.WriteFile(agentCertPath, agent.CertPEM, ctx.Log); err != nil {
				return err
			}
			return certs.WritePrivateKey(agentKeyPath, agent.KeyPEM, ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Building Dashboard Image",
		"Building the dashboard container for monitoring SPIRE state.",
		func() error {
			containerfile := filepath.Join(ctx.RepoRoot, "dashboard", "Containerfile")
			return podman.Build("spiffe-spire-demo-dashboard:local", containerfile, ctx.RepoRoot, ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting SPIRE Server and Agent",
		"Both the server and agent start together. Unlike scenario 01 where the\n"+
			"agent needed a join token, here the agent automatically attests using\n"+
			"its x509pop certificate. The server validates the cert chain against\n"+
			"the trusted CA we provisioned earlier.\n\n"+
			"No shared secret (token) is needed - the trust is established through\n"+
			"the PKI hierarchy, just like real TPM-based attestation.",
		func() error {
			return ctx.Compose.Up("spire-server", "spire-agent")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Waiting for Containers",
		"Waiting for both SPIRE server and agent containers to be running.",
		func() error {
			time.Sleep(5 * time.Second)
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Waiting for x509pop Attestation",
		"The agent presents its certificate to the server. The server:\n"+
			"  1. Validates the certificate signature against the trusted CA\n"+
			"  2. Verifies the agent possesses the corresponding private key\n"+
			"  3. Registers the agent with attestation type 'x509pop'\n\n"+
			"This is more secure than join tokens because the private key never\n"+
			"leaves the agent (in real TPM scenarios, it never leaves the hardware).",
		func() error {
			_, err := spirectl.WaitForAgent(ctx.Compose, "spire-server", "x509pop", ctx.Log)
			return err
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting Dashboard",
		"Starting the dashboard to view the attested agent.",
		func() error {
			if err := ctx.Compose.UpNoBuild("dashboard"); err != nil {
				return err
			}
			ctx.WaitForDashboard("http://127.0.0.1:8080/health")
			return nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func done(ctx *scenario.Context) {
	ctx.Log.Step("TPM scenario is ready!")
	ctx.Log.Info("Dashboard: http://localhost:8080")
	ctx.Log.Info("")
	ctx.Log.Info("Key difference from scenario 01:")
	ctx.Log.Info("  - No join token was needed")
	ctx.Log.Info("  - Agent proved identity via certificate (x509pop)")
	ctx.Log.Info("  - In production, this certificate lives in a TPM chip")
	ctx.Log.Info("")
	ctx.Log.Info("Tear down with: scenario-04-tpm down")
}
