package setup

const (
	beginMarker = "# >>> sv managed - do not edit, changes will be overwritten >>>"
	endMarker   = "# <<< sv managed <<<"
)

const bashBlock = `shopt -s histappend
HISTSIZE=100000
HISTFILESIZE=200000
HISTCONTROL=ignoreboth
HISTTIMEFORMAT='%F %T '
case ";$PROMPT_COMMAND;" in
  *"history -a"*) ;;
  *) PROMPT_COMMAND="history -a${PROMPT_COMMAND:+; $PROMPT_COMMAND}" ;;
esac`

const zshBlock = `setopt INC_APPEND_HISTORY EXTENDED_HISTORY HIST_IGNORE_DUPS
HISTSIZE=100000
SAVEHIST=100000
: "${HISTFILE:=$HOME/.zsh_history}"`

const Script = `#!/bin/sh
set -e
shell=$(basename "${SHELL:-/bin/sh}")
case "$shell" in
  bash) rc=$HOME/.bashrc ;;
  zsh) rc=$HOME/.zshrc ;;
  *) echo "sv: login shell is $shell, only bash and zsh are supported" >&2; exit 2 ;;
esac
tmp=$rc.sv-tmp
if [ -f "$rc" ]; then
  awk '/^` + beginMarker + `$/{skip=1} /^` + endMarker + `$/{skip=0; next} skip{next} /^$/{blank++; next} {for(;blank>0;blank--) print ""; print}' "$rc" > "$tmp"
  [ -s "$tmp" ] && echo >> "$tmp"
else
  : > "$tmp"
fi
{
  echo '` + beginMarker + `'
  case "$shell" in
    bash) cat <<'SV_BASH'
` + bashBlock + `
SV_BASH
      ;;
    zsh) cat <<'SV_ZSH'
` + zshBlock + `
SV_ZSH
      ;;
  esac
  echo '` + endMarker + `'
} >> "$tmp"
mv "$tmp" "$rc"
echo "history: $shell, block written to ~/$(basename "$rc")"
if command -v tmux >/dev/null 2>&1; then
  echo "tmux: $(tmux -V)"
else
  echo "tmux: not installed, sessions will not survive disconnects"
fi
`
