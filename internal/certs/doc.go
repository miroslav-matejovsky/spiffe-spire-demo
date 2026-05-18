// Package certs generates self-signed X.509 certificates for SPIRE agent
// attestation demos. It replaces the PowerShell scripts that used .NET crypto
// (System.Security.Cryptography) to provision x509pop and DevID materials.
//
// The certificates generated here are for demonstration purposes only. They use
// RSA 2048-bit keys and are valid for 365 days.
//
// Two certificate types are generated:
//   - CA certificate: self-signed root used by the SPIRE server to verify agent certs
//   - Agent certificate: signed by the CA, used by the SPIRE agent for x509pop attestation
//
// The CA certificate goes into the SPIRE server config directory, and both the
// agent certificate and private key go into the SPIRE agent config directory.
package certs
