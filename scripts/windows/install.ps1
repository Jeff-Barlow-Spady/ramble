#Requires -RunAsAdministrator

function Write-ColorOutput($ForegroundColor)
{
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    else {
        $input | Write-Output
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Write-Success($message) {
    Write-ColorOutput Green "✓ $message"
}

function Write-Warning($message) {
    Write-ColorOutput Yellow "⚠️ $message"
}

function Write-Error($message) {
    Write-ColorOutput Red "❌ $message"
    exit 1
}

function Test-CommandExists {
    param ($command)
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'stop'
    try { if (Get-Command $command) { return $true } }
    catch { return $false }
    finally { $ErrorActionPreference = $oldPreference }
}

# Print header
Write-Host "=== Ramble Speech-to-Text Installer for Windows ===" -ForegroundColor Cyan
Write-Host ""

# Get paths
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ParentDir = (Get-Item $ScriptDir).Parent.Parent.FullName
$DistDir = Join-Path -Path $ParentDir -ChildPath "dist\windows"

# Set installation directories
$AppDir = Join-Path -Path $env:LOCALAPPDATA -ChildPath "Ramble"
$ModelsDir = Join-Path -Path $AppDir -ChildPath "models"
$LogsDir = Join-Path -Path $AppDir -ChildPath "logs"

Write-Host "Creating application directories..." -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path $AppDir | Out-Null
New-Item -ItemType Directory -Force -Path $ModelsDir | Out-Null
New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null

# Copy application files
Write-Host "Installing Ramble application..." -ForegroundColor Cyan

$LibDir = Join-Path -Path $DistDir -ChildPath "libs"
if (Test-Path $LibDir) {
    Copy-Item -Path "$LibDir\*" -Destination "$AppDir\libs" -Recurse -Force
    Write-Success "Libraries installed successfully"
} else {
    Write-Error "Required libraries not found at $LibDir"
}

$ExecutablePath = Join-Path -Path $DistDir -ChildPath "ramble.exe"
if (Test-Path $ExecutablePath) {
    Copy-Item -Path $ExecutablePath -Destination $AppDir -Force
    Write-Success "Executable installed successfully"
} else {
    Write-Error "Executable not found at $ExecutablePath"
}

# Copy models if they exist
$ModelsSourceDir = Join-Path -Path $DistDir -ChildPath "models"
if (Test-Path $ModelsSourceDir) {
    if ((Get-ChildItem -Path $ModelsSourceDir).Count -gt 0) {
        Write-Host "Installing default speech models..." -ForegroundColor Cyan
        Copy-Item -Path "$ModelsSourceDir\*" -Destination $ModelsDir -Recurse -Force
        Write-Success "Models installed successfully"
    } else {
        Write-Warning "No model files found in the models directory"
    }
} else {
    Write-Warning "No bundled models found. You'll need to download models through the application preferences."
}

# Create icon if it exists
$IconPath = Join-Path -Path $DistDir -ChildPath "ramble.ico"
if (-not (Test-Path $IconPath)) {
    $IconPath = Join-Path -Path $AppDir -ChildPath "ramble.exe"
    Write-Warning "Icon not found, using executable icon instead"
}

# Create desktop shortcut
Write-Host "Creating desktop shortcuts and start menu entries..." -ForegroundColor Cyan
$WshShell = New-Object -ComObject WScript.Shell
$DesktopPath = [System.Environment]::GetFolderPath('Desktop')
$Shortcut = $WshShell.CreateShortcut("$DesktopPath\Ramble.lnk")
$Shortcut.TargetPath = Join-Path -Path $AppDir -ChildPath "ramble.exe"
$Shortcut.WorkingDirectory = $AppDir
$Shortcut.Description = "Speech-to-Text Transcription"
$Shortcut.IconLocation = $IconPath
$Shortcut.Save()
Write-Success "Desktop shortcut created"

# Create start menu entry
$StartMenuDir = Join-Path -Path $env:APPDATA -ChildPath "Microsoft\Windows\Start Menu\Programs\Ramble"
New-Item -ItemType Directory -Force -Path $StartMenuDir | Out-Null
$Shortcut = $WshShell.CreateShortcut("$StartMenuDir\Ramble.lnk")
$Shortcut.TargetPath = Join-Path -Path $AppDir -ChildPath "ramble.exe"
$Shortcut.WorkingDirectory = $AppDir
$Shortcut.Description = "Speech-to-Text Transcription"
$Shortcut.IconLocation = $IconPath
$Shortcut.Save()
Write-Success "Start menu entry created"

# Add to PATH
Write-Host "Adding application to PATH..." -ForegroundColor Cyan
$UserPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if (-not $UserPath.Contains($AppDir)) {
    [System.Environment]::SetEnvironmentVariable("PATH", "$UserPath;$AppDir", "User")
    Write-Success "Added Ramble to user PATH"
} else {
    Write-Warning "Ramble already in PATH, skipping"
}

Write-Host ""
Write-Host "=== Installation Complete ===" -ForegroundColor Green
Write-Host "Ramble has been installed to: $AppDir" -ForegroundColor White
Write-Host "Models are stored in: $ModelsDir" -ForegroundColor White
Write-Host "Logs are stored in: $LogsDir" -ForegroundColor White
Write-Host ""
Write-Host "You can now run Ramble from:" -ForegroundColor White
Write-Host "- The desktop shortcut" -ForegroundColor White
Write-Host "- The Start Menu" -ForegroundColor White
Write-Host "- Command prompt by typing 'ramble'" -ForegroundColor White
Write-Host ""
Write-Host "Enjoy using Ramble!" -ForegroundColor Cyan
Write-Host ""
Read-Host -Prompt "Press Enter to exit"