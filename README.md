# teletrack

Displays current playing Spotify track in Telegram channel post.

![screenshot of teletrack](./screenshot.png)

## Install

1. Run `teletrack` for first time. `config.json` will be created.
2. Get [last.fm API token](https://www.last.fm/api).
3. Get [Spotify API token](https://developer.spotify.com/dashboard).
4. Create [Telegram Bot](https://t.me/botfather).
5. Fill `config.json`. `telegram`, `lastFm` fields, `spotify` (except `token` field).
6. Run `teletrack`, and authorize `Spotify` (see messages in console).

Automized deployment (to VPS, for example) can be achivied via [ansiblecfgs](https://github.com/oklookat/ansiblecfgs/tree/v2/playbooks/teletrack).

## Flags

`-c`: path to config file. Default: `config.json`.
