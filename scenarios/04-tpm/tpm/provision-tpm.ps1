$ErrorActionPreference = "Stop"

$scenarioRoot = Split-Path -Parent $PSScriptRoot
$serverDir = Join-Path $scenarioRoot "spire\server"
$agentDir = Join-Path $scenarioRoot "spire\agent"

Write-Host "Provisioning demo DevID materials..." -ForegroundColor Cyan

$caCertPath = Join-Path $serverDir "devid-ca.pem"
$caKeyPath = Join-Path $serverDir "devid-ca-key.pem"
$agentKeyPath = Join-Path $agentDir "devid-key.pem"
$agentCertPath = Join-Path $agentDir "devid-cert.pem"

$generatedFiles = @(
    $caCertPath,
    $caKeyPath,
    $agentKeyPath,
    $agentCertPath
)

foreach ($file in $generatedFiles) {
    if (Test-Path $file) {
        Remove-Item $file -Force
    }
}

$rsaKeySize = 2048
$notBefore = [System.DateTimeOffset]::UtcNow.AddMinutes(-5)
$notAfter = $notBefore.AddDays(365)

$caKey = [System.Security.Cryptography.RSA]::Create($rsaKeySize)
$caRequest = [System.Security.Cryptography.X509Certificates.CertificateRequest]::new(
    "CN=Demo DevID CA",
    $caKey,
    [System.Security.Cryptography.HashAlgorithmName]::SHA256,
    [System.Security.Cryptography.RSASignaturePadding]::Pkcs1
)
$caRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509BasicConstraintsExtension]::new($true, $false, 0, $true))
$caRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509KeyUsageExtension]::new([System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::KeyCertSign -bor [System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::CrlSign, $true))
$caRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509SubjectKeyIdentifierExtension]::new($caRequest.PublicKey, $false))
$caCertificate = $caRequest.CreateSelfSigned($notBefore, $notAfter)

$agentKey = [System.Security.Cryptography.RSA]::Create($rsaKeySize)
$agentRequest = [System.Security.Cryptography.X509Certificates.CertificateRequest]::new(
    "CN=SPIRE Agent DevID",
    $agentKey,
    [System.Security.Cryptography.HashAlgorithmName]::SHA256,
    [System.Security.Cryptography.RSASignaturePadding]::Pkcs1
)
$agentRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509BasicConstraintsExtension]::new($false, $false, 0, $true))
$agentRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509KeyUsageExtension]::new([System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::DigitalSignature -bor [System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::KeyEncipherment, $true))
$enhancedKeyUsage = [System.Security.Cryptography.OidCollection]::new()
$null = $enhancedKeyUsage.Add([System.Security.Cryptography.Oid]::new("1.3.6.1.5.5.7.3.2", "Client Authentication"))
$agentRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509EnhancedKeyUsageExtension]::new($enhancedKeyUsage, $false))
$agentRequest.CertificateExtensions.Add([System.Security.Cryptography.X509Certificates.X509SubjectKeyIdentifierExtension]::new($agentRequest.PublicKey, $false))
$serialNumber = New-Object byte[] 16
[System.Security.Cryptography.RandomNumberGenerator]::Fill($serialNumber)
$agentCertificate = $agentRequest.Create($caCertificate, $notBefore, $notAfter, $serialNumber)

Set-Content -Path $caCertPath -Value $caCertificate.ExportCertificatePem() -NoNewline
Set-Content -Path $caKeyPath -Value $caKey.ExportPkcs8PrivateKeyPem() -NoNewline
Set-Content -Path $agentCertPath -Value $agentCertificate.ExportCertificatePem() -NoNewline
Set-Content -Path $agentKeyPath -Value $agentKey.ExportPkcs8PrivateKeyPem() -NoNewline

$caCertificate.Dispose()
$agentCertificate.Dispose()
$caKey.Dispose()
$agentKey.Dispose()

Write-Host "Provisioning complete." -ForegroundColor Green
Write-Host "  CA certificate:    $caCertPath" -ForegroundColor Gray
Write-Host "  CA private key:    $caKeyPath" -ForegroundColor Gray
Write-Host "  Agent certificate: $agentCertPath" -ForegroundColor Gray
Write-Host "  Agent private key: $agentKeyPath" -ForegroundColor Gray
