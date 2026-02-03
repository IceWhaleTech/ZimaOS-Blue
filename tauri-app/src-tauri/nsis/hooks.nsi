; ZimaOS Echo - NSIS Pre-install hooks
; Custom branding and simplified installation

!macro NSIS_HOOK_PREINSTALL
  DetailPrint "Installing ZimaOS Echo..."
!macroend

!macro NSIS_HOOK_POSTINSTALL
  DetailPrint "Installation complete!"
!macroend
