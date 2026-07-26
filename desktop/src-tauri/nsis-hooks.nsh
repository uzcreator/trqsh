; trqsh NSIS installer hooks.
;
; The desktop app runs the Go agent (trqsh.exe) as a background sidecar and, by
; design, keeps it alive when the window is closed (tray / background mode). On an
; update the installer then can't overwrite the running trqsh.exe and fails with
;   "Error opening file for writing: ...\trqsh.exe"
; leaving users on a half-updated install. So before installing OR uninstalling we
; force-stop the agent and the app, then give Windows a moment to release the file
; handles. taskkill on an absent process just returns non-zero, which we discard.

!macro NSIS_HOOK_PREINSTALL
  nsExec::Exec 'taskkill /F /T /IM trqsh-desktop.exe'
  Pop $0
  nsExec::Exec 'taskkill /F /T /IM trqsh.exe'
  Pop $0
  Sleep 1000
!macroend

!macro NSIS_HOOK_PREUNINSTALL
  nsExec::Exec 'taskkill /F /T /IM trqsh-desktop.exe'
  Pop $0
  nsExec::Exec 'taskkill /F /T /IM trqsh.exe'
  Pop $0
  Sleep 1000
!macroend
