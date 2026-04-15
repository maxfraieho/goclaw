# Multi-Profile Survey Agents Setup

Date: 2026-04-15
Host: Alpine (192.168.3.184)

## Overview

Created separate GoClaw agents for Olena and Arsen survey profiles.

## Agents Created

| Agent Key | ID | Profile |
|-----------|-----|---------|
| surveys-olena | 019d8fda-dd8f-7668-9f42-ff4641b6128d | Annet Buonassie (52 y.o.) |
| surveys-arsen | 019d8fda-e3b2-7d4c-9fc7-7bbda586562e | Arno Dubois (25 y.o.) |

## Profile Differences

### Olena (Annet Buonassie)
- Age: 52
- Email: lekov00@gmail.com
- Always runs on Alpine
- VPN: Swiss SOCKS5 (100.100.74.9:9888)
- Insurance: la Mubilière + Helsana
- Bank: Revolut
- Scores: 7/10, Agreement: Oui, plutot daccord

### Arsen (Arno Dubois)
- Age: 25
- Email: arsen.k111999@gmail.com
- Runs on Alpine as FALLBACK (primary on MacBook)
- VPN: Swiss SOCKS5 (100.100.74.9:9888)  
- Insurance: Helvetia/Allianz (auto), Helsana (medical)
- Bank: UBS/BCV main, Revolut secondary
- FINANCIAL INDEPENDENCE: Always Je paie moi-meme
- Scores: 7-8/10

## Workspace Files

Each agent has:
- SOUL.md - personality and profile data
- AGENTS.md - agent instructions and rules
- USER.md - operator notes and credentials

## Channel Instances

| Agent | Channel Name | Telegram Bot |
|-------|--------------|--------------|
| surveys-olena | telegram-olena | Bot_for_Olena |
| surveys-arsen | telegram-arsen | Bot_for_Arsen |

## Hard Rules

1. NEVER combine answer selection and Continuer in same action
2. ALWAYS verify Swiss VPN before opening survey (ipinfo.io/country = CH)
3. Open surveys ONLY via PinchTab with Swiss proxy
4. Stop only on critical technical error
5. For Arsen: financial independence - ALWAYS Je paie moi-meme

## Files Created

- /home/vokov/.goclaw/workspace/surveys-olena/{SOUL,AGENTS,USER}.md
- /home/vokov/.goclaw/workspace/surveys-arsen/{SOUL,AGENTS,USER}.md

## Git Commit

Branch: fix/pinchtab-stop-recovery
