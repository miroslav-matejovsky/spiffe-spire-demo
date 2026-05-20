// Command scenario-04-tpm demonstrates TPM-style attestation using x509pop
// (X.509 Proof of Possession). Instead of a join token, the agent proves its
// identity by possessing a certificate signed by a trusted CA.
//
// This scenario provisions demo certificates (CA + agent cert) and configures
// SPIRE to use x509pop attestation. It shows how hardware-backed identity
// (like TPM DevID certificates) can bootstrap trust without shared secrets.
//
// Usage:
//
//	scenario-04-tpm [flags]
//	  --step     Pause between steps for interactive learning
//	  --verbose  Show detailed debug output
//	  --down     Tear down the scenario instead of starting it
package main
