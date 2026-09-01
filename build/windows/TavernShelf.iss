#define AppName "Tavern Shelf"
#define AppVersion "0.1.0"
#define AppPublisher "Tavern Player"
#define AppExeName "TavernShelf.exe"

[Setup]
AppId={{F39E7992-1513-4A7A-B766-9D3D77BA9371}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
DefaultDirName={localappdata}\Programs\Tavern Shelf
DefaultGroupName=Tavern Shelf
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputDir=..\installer
OutputBaseFilename=TavernShelf-Setup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\{#AppExeName}
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible

[Files]
Source: "..\bin\TavernShelf.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Tavern Shelf"; Filename: "{app}\{#AppExeName}"
Name: "{userdesktop}\Tavern Shelf"; Filename: "{app}\{#AppExeName}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "创建桌面快捷方式"; GroupDescription: "附加快捷方式："; Flags: unchecked

[Run]
Filename: "{app}\{#AppExeName}"; Description: "启动 Tavern Shelf"; Flags: nowait postinstall skipifsilent
