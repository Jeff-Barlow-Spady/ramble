@echo off
setlocal enabledelayedexpansion

echo === Ramble Speech-to-Text Installer for Windows ===
echo.

:: Get script directory
set "SCRIPT_DIR=%~dp0"
set "PARENT_DIR=%SCRIPT_DIR%..\..\"
set "PARENT_DIR=%PARENT_DIR:\=/%"

:: Set installation directories
set "APP_DIR=%LOCALAPPDATA%\Ramble"
set "MODELS_DIR=%APP_DIR%\models"
set "LOGS_DIR=%APP_DIR%\logs"

echo Creating application directories...
if not exist "%APP_DIR%" mkdir "%APP_DIR%"
if not exist "%MODELS_DIR%" mkdir "%MODELS_DIR%"
if not exist "%LOGS_DIR%" mkdir "%LOGS_DIR%"

:: Copy application files
echo Installing Ramble application...
if exist "%PARENT_DIR%\dist\windows\libs" (
    xcopy /E /I /Y "%PARENT_DIR%\dist\windows\libs" "%APP_DIR%\libs"
) else (
    echo Error: Required libraries not found.
    exit /b 1
)

if exist "%PARENT_DIR%\dist\windows\ramble.exe" (
    copy "%PARENT_DIR%\dist\windows\ramble.exe" "%APP_DIR%\"
    echo Executable installed successfully.
) else (
    echo Error: Executable not found.
    exit /b 1
)

:: Copy models if they exist
if exist "%PARENT_DIR%\dist\windows\models\*" (
    echo Installing default speech models...
    xcopy /E /I /Y "%PARENT_DIR%\dist\windows\models\*" "%MODELS_DIR%\"
    echo Models installed successfully.
) else (
    echo No bundled models found. You'll need to download models through the application preferences.
)

:: Create launcher
echo @echo off > "%APP_DIR%\ramble.bat"
echo cd /d "%APP_DIR%" >> "%APP_DIR%\ramble.bat"
echo start "" ramble.exe %%* >> "%APP_DIR%\ramble.bat"

:: Create shortcut on desktop
echo Creating desktop shortcut...
powershell -Command "$WshShell = New-Object -comObject WScript.Shell; $Shortcut = $WshShell.CreateShortcut([System.Environment]::GetFolderPath('Desktop') + '\Ramble.lnk'); $Shortcut.TargetPath = '%APP_DIR%\ramble.exe'; $Shortcut.WorkingDirectory = '%APP_DIR%'; $Shortcut.Description = 'Speech-to-Text Transcription'; $Shortcut.Save()"

:: Create start menu shortcut
echo Creating start menu entry...
set "START_MENU_DIR=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Ramble"
if not exist "%START_MENU_DIR%" mkdir "%START_MENU_DIR%"
powershell -Command "$WshShell = New-Object -comObject WScript.Shell; $Shortcut = $WshShell.CreateShortcut('%START_MENU_DIR%\Ramble.lnk'); $Shortcut.TargetPath = '%APP_DIR%\ramble.exe'; $Shortcut.WorkingDirectory = '%APP_DIR%'; $Shortcut.Description = 'Speech-to-Text Transcription'; $Shortcut.Save()"

:: Add to PATH
echo Adding application to PATH...
setx PATH "%PATH%;%APP_DIR%" /M

echo.
echo === Installation Complete ===
echo Ramble has been installed to: %APP_DIR%
echo Models are stored in: %MODELS_DIR%
echo Logs are stored in: %LOGS_DIR%
echo.
echo You can now run Ramble from:
echo - The desktop shortcut
echo - The Start Menu
echo - Command prompt by typing "ramble"
echo.
echo Enjoy using Ramble!
echo.
pause