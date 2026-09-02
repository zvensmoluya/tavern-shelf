#define AppName "Tavern Shelf"
#ifndef AppVersion
#define AppVersion "0.1.0"
#endif
#define AppPublisher "Tavern Shelf"
#define AppExeName "TavernShelf.exe"

[Setup]
AppId={{F39E7992-1513-4A7A-B766-9D3D77BA9371}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL=https://github.com/zvensmoluya/tavern-shelf
AppSupportURL=https://github.com/zvensmoluya/tavern-shelf/issues
AppUpdatesURL=https://github.com/zvensmoluya/tavern-shelf/releases
AppCopyright=Copyright (c) 2026 zvensmoluya
DefaultDirName={localappdata}\Programs\Tavern Shelf
DefaultGroupName=Tavern Shelf
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputDir=..\installer
OutputBaseFilename=TavernShelf-Setup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
LicenseFile=..\..\LICENSE
UninstallDisplayIcon={app}\TavernShelf.ico
SetupIconFile=TavernShelf.ico
VersionInfoVersion={#AppVersion}
VersionInfoCompany=Tavern Shelf
VersionInfoDescription=Tavern Shelf Windows Installer
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible

[Files]
Source: "..\bin\TavernShelf.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "TavernShelf.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Tavern Shelf"; Filename: "{app}\{#AppExeName}"; IconFilename: "{app}\TavernShelf.ico"
Name: "{userdesktop}\Tavern Shelf"; Filename: "{app}\{#AppExeName}"; IconFilename: "{app}\TavernShelf.ico"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "创建桌面快捷方式"; GroupDescription: "附加快捷方式："; Flags: unchecked

[Run]
Filename: "{app}\{#AppExeName}"; Description: "启动 Tavern Shelf"; Flags: nowait postinstall skipifsilent
