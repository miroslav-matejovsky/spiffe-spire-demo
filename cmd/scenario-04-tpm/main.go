package main

import (
	"path/filepath"
	"time"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/certs"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/step"
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

	err := ctx.Runner.RunStep(step.Step{
		Name: "Provisioning x509pop Certificates",
		Explain: "TPM is hardware security module. It stores identity key and signs without giving key away.\n" +
			"Real devices often carry DevID certificate from IEEE 802.1AR manufacturing flow.\n" +
			"For demo, x509pop stands in for TPM DevID attestation with normal PEM files.\n" +
			"We build small PKI: CA signs agent certificate, and SPIRE server trusts only CA.\n" +
			"Server gets CA certificate as trust anchor. Agent gets certificate plus private key.\n" +
			"In real TPM flow, private key is hardware-bound and non-extractable, but trust model is same.",
		Action: func() error {
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
		Observe: "Three files now exist.\n" +
			"devid-ca.pem goes to SPIRE server as trusted issuing CA.\n" +
			"devid-cert.pem is agent identity certificate presented during attestation.\n" +
			"devid-key.pem proves possession of certificate key.\n" +
			"In production, key would stay inside TPM chip and never be written to disk.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Building Dashboard Image",
		Explain: "Dashboard is side tool for watching SPIRE state.\n" +
			"We build image now so later steps can show agent record and attestation type.\n" +
			"No TPM logic here, only user view into server data.",
		Action: func() error {
			containerfile := filepath.Join(ctx.RepoRoot, "dashboard", "Containerfile")
			return podman.Build("spiffe-spire-demo-dashboard:local", containerfile, ctx.RepoRoot, ctx.Log)
		},
		Observe: "Dashboard image is ready.\n" +
			"Later container start will be fast because image already exists locally.\n" +
			"Use it to inspect agent details without running spire-server commands by hand.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting SPIRE Server and Agent",
		Explain: "Scenario 01 used join token, which is one-time shared secret passed at startup.\n" +
			"Here no runtime secret is needed because trust comes from certificate chain.\n" +
			"Server config enables x509pop NodeAttestor and points ca_bundle_path to devid-ca.pem.\n" +
			"Agent config enables x509pop and points to devid-cert.pem plus devid-key.pem.\n" +
			"Both containers can start together because agent already has long-lived identity material.\n" +
			"Real TPM attestation swaps file-based key for hardware-backed key, but flow is similar.",
		Action: func() error {
			return ctx.Compose.Up("spire-server", "spire-agent")
		},
		Observe: "Both containers are booting now.\n" +
			"SPIRE server loaded trusted CA bundle and is ready to validate presented chain.\n" +
			"SPIRE agent will present certificate and prove it owns matching private key.\n" +
			"No one-time join token moves over network or command line.\n" +
			"Trust is being established through PKI instead of shared secret.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for Containers",
		Explain: "Short pause for container health checks.\n" +
			"Server must finish boot before it can verify x509pop requests.\n" +
			"Agent can start earlier, but attestation succeeds only after server is healthy.",
		Action: func() error {
			time.Sleep(5 * time.Second)
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
		Observe: "Containers should now be running.\n" +
			"Healthcheck confirms SPIRE server API is up and ready for node attestation traffic.\n" +
			"Next step can focus on x509pop exchange instead of startup noise.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for x509pop Attestation",
		Explain: "Now agent performs node attestation with x509pop proof of possession.\n" +
			"Agent sends certificate chain rooted in trusted DevID-style CA.\n" +
			"Server validates chain against configured CA bundle and checks certificate details.\n" +
			"Agent also proves it owns private key matching leaf certificate.\n" +
			"If checks pass, SPIRE records attestation type as x509pop, not join_token.\n" +
			"Result is same bootstrap goal as TPM DevID: identity comes from PKI-backed device proof.",
		Action: func() error {
			_, err := spirectl.WaitForAgent(ctx.Compose, "spire-server", "x509pop", ctx.Log)
			return err
		},
		Observe: "Agent should now be attested via x509pop.\n" +
			"Compare with scenario 01: attestation type is x509pop instead of join_token.\n" +
			"Agent SPIFFE ID path also contains x509pop instead of join_token.\n" +
			"Try: podman-compose exec spire-server /opt/spire/bin/spire-server agent list\n" +
			"Look for attestation data and selector path tied to x509pop flow.\n" +
			"Main lesson: no shared bootstrap secret was needed.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Dashboard",
		Explain: "Dashboard starts after attestation so data is already visible.\n" +
			"It reads SPIRE server state and shows agent records in browser.\n" +
			"Use it to connect config, attestation event, and final identity view.",
		Action: func() error {
			if err := ctx.Compose.UpNoBuild("dashboard"); err != nil {
				return err
			}
			ctx.WaitForDashboard("http://127.0.0.1:8080/health")
			return nil
		},
		Observe: "Dashboard is live at http://localhost:8080.\n" +
			"Open Agents page and inspect attestation type for TPM demo agent.\n" +
			"It should show x509pop, not join_token.\n" +
			"Compare mentally with scenario 01 and note how trust source changed.",
	})
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
