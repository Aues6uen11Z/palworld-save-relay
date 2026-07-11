Place real Palworld .sav fixtures here for round-trip tests (gitignored).
Run fetch.ps1 to copy them from a local Palworld install.

Required files:
  level_plz.sav     - a PlZ (zlib) Level.sav
  player_plz.sav    - a PlZ player .sav
  level_plm.sav     - a PlM (Oodle) Level.sav
  player_plm.sav    - a PlM player .sav
  localdata_plm.sav - a PlM LocalData.sav (optional)

Tests in package sav skip automatically if any fixture is missing.
