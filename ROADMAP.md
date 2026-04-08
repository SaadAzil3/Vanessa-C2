# Vanissa C2 — Roadmap & Learning Path

## ✅ Done
- Multi-agent registry with CHECKIN
- Multi-channel (Telegram + Discord) with channel switching
- Operator console (interactive shell + Flask API)
- Dropper with decoy PDF + RLO masquerading

## 🔧 Next Features (pick in order)
1. **Persistence on target** — survive reboot (registry run key, scheduled task)
2. **Encryption** — AES-encrypt protocol messages
3. **File exfil/upload** — download/upload files from target
4. **SQLite logging** — persist agent sessions + command history
5. **Screenshot module** — capture target screen
6. **LoDD channel** — LLM-obfuscated dead drop (Claude + Notion) for thesis

---

## 📚 What to Learn

| Topic | Resource | Why |
|-------|----------|-----|
| Windows Internals | *Windows Internals* by Russinovich | Understand processes, tokens, registry |
| Malware Dev in Go | [Red Team Development with Go](https://github.com/redcode-labs) | Your agent is in Go |
| MITRE ATT&CK | attack.mitre.org | Map your techniques properly for thesis |
| C2 Frameworks | Study Sliver, Cobalt Strike, Mythic | See how pros build C2 |
| Evasion | AMSI bypass, ETW patching, unhooking | Bypass AV/EDR |
| Cryptography | AES-GCM, TLS pinning, key exchange | Secure comms |
| Persistence | T1547, T1053 (ATT&CK) | Survive reboot |
| Process Injection | DLL injection, shellcode, hollowing | Advanced post-exploitation |

## 🏠 HomeLabs to Build

### Lab 1: Detection Lab
- Windows 10/11 VM + Sysmon + Splunk/ELK
- Run your agent, analyze what gets logged
- Learn what defenders see → learn to evade

### Lab 2: AV Evasion Lab
- Windows Defender enabled
- Test your dropper, get flagged, learn to bypass
- Techniques: obfuscation, syscall stubs, signing

### Lab 3: Active Directory Lab
- Windows Server 2019 DC + 2 workstations
- Test lateral movement with your C2
- Kerberoasting, pass-the-hash, DCSync

### Lab 4: Network Monitoring Lab
- Wireshark + Suricata on your Linux host
- Capture your C2 traffic → learn what triggers alerts
- Add encryption + domain fronting to evade

### Lab 5: Incident Response Lab
- Infect your VM, then switch to blue team
- Use Volatility (memory forensics) + Autopsy (disk)
- Document the full kill chain for your thesis

---

## 🎯 Thesis Priorities
1. Get the C2 working end-to-end (✅ nearly there)
2. Implement LoDD channel (your novel contribution)
3. Build Lab 1 — run your C2, capture Sysmon logs
4. Write the detection vs evasion analysis
