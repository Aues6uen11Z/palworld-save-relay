package palworld

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// SteamAccount is a Steam account that has Palworld save data on this machine.
type SteamAccount struct {
	SteamID     string `json:"steamId"`
	PersonaName string `json:"personaName"`
	AccountName string `json:"accountName"`
	MostRecent  bool   `json:"mostRecent"`
}

// ListSteamAccounts lists Steam accounts that have Palworld save folders,
// enriched with display names from Steam's loginusers.vdf. Accounts with saves
// but no loginusers entry (e.g. not logged in) are still listed, showing just
// the numeric ID.
func ListSteamAccounts(saveRoot string) ([]SteamAccount, error) {
	// 1. Collect SteamID folders that exist under the save root.
	saveIDs := map[string]bool{}
	entries, err := os.ReadDir(saveRoot)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var sid uint64
		if _, err := fmt.Sscanf(e.Name(), "%d", &sid); err == nil && sid > 0 {
			saveIDs[e.Name()] = true
		}
	}

	// 2. Read loginusers.vdf for display names.
	vdf := parseLoginUsers()

	// 3. Merge.
	var accounts []SteamAccount
	for sid := range saveIDs {
		acc := SteamAccount{SteamID: sid}
		if info, ok := vdf[sid]; ok {
			acc.PersonaName = info.PersonaName
			acc.AccountName = info.AccountName
			acc.MostRecent = info.MostRecent
		}
		accounts = append(accounts, acc)
	}
	// Sort: MostRecent first, then by PersonaName.
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].MostRecent != accounts[j].MostRecent {
			return accounts[i].MostRecent
		}
		return accounts[i].PersonaName < accounts[j].PersonaName
	})
	return accounts, nil
}

// parseLoginUsers reads Steam's config/loginusers.vdf and returns a map of
// SteamID64 -> SteamAccount (with PersonaName, AccountName, MostRecent).
// Returns an empty map if the file is not found or unparseable.
func parseLoginUsers() map[string]SteamAccount {
	result := map[string]SteamAccount{}
	steamPath, err := steamInstallPath()
	if err != nil {
		return result
	}
	f, err := os.Open(filepath.Join(steamPath, "config", "loginusers.vdf"))
	if err != nil {
		return result
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var currentID string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// A SteamID64 key line: "76561..." (always starts with "76561")
		if strings.HasPrefix(line, `"76561`) && strings.HasSuffix(line, `"`) {
			currentID = strings.Trim(line, `"`)
			continue
		}
		if currentID == "" {
			continue
		}
		if strings.HasPrefix(line, `"PersonaName"`) {
			acc := result[currentID]
			acc.SteamID = currentID
			acc.PersonaName = vdfValue(line, "PersonaName")
			result[currentID] = acc
		}
		if strings.HasPrefix(line, `"AccountName"`) {
			acc := result[currentID]
			acc.SteamID = currentID
			acc.AccountName = vdfValue(line, "AccountName")
			result[currentID] = acc
		}
		if strings.HasPrefix(line, `"MostRecent"`) {
			acc := result[currentID]
			acc.SteamID = currentID
			acc.MostRecent = vdfValue(line, "MostRecent") == "1"
			result[currentID] = acc
		}
		if line == "}" {
			currentID = ""
		}
	}
	return result
}

// vdfValue extracts the right-hand value from a VDF line like
// `"PersonaName"    "Aues6uen11Z"`.
func vdfValue(line, key string) string {
	s := strings.TrimPrefix(line, `"`+key+`"`)
	s = strings.TrimSpace(s)
	return strings.Trim(s, `"`)
}

// steamInstallPath returns the Steam install directory via the Windows
// registry. Falls back to C:\Program Files (x86)\Steam if not found.
func steamInstallPath() (string, error) {
	c := exec.Command("reg", "query", `HKCU\Software\Valve\Steam`, "/v", "SteamPath")
	hideCmdWindow(c)
	out, err := c.Output()
	if err != nil {
		return `C:\Program Files (x86)\Steam`, nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, "REG_SZ"); idx >= 0 {
			return strings.TrimSpace(line[idx+6:]), nil
		}
	}
	return `C:\Program Files (x86)\Steam`, nil
}
