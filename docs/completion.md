# Shell Completion

`parsec` supports generating autocompletion scripts for various shells. This allows you to use the `Tab` key to complete commands, flags, and arguments.

## Supported Shells

- [Bash](#bash)
- [Fish](#fish)
- [Zsh](#zsh)
- [PowerShell](#powershell)

---

## Bash

The generated script depends on the `bash-completion` package. If it is not installed already, you can install it via your OS's package manager (e.g., `sudo apt install bash-completion` on Ubuntu or `brew install bash-completion` on macOS).

### Load in Current Session
```bash
source <(parsec completion bash)
```

### Permanent Setup

```bash
mkdir -p ~/.local/share/bash-completion/completions
parsec completion bash > ~/.local/share/bash-completion/completions/parsec
```
Note: You may need to add `source ~/.local/share/bash-completion/completions/parsec` to your `~/.bashrc` if your system doesn't automatically load completions from this directory.

---

## Fish

### Load in Current Session
```fish
parsec completion fish | source
```

### Permanent Setup
```fish
parsec completion fish > ~/.config/fish/completions/parsec.fish
```

---

## Zsh

If shell completion is not already enabled in your environment, you will need to enable it. You can execute the following once:

```zsh
echo "autoload -U compinit; compinit" >> ~/.zshrc
```

### Load in Current Session
```zsh
source <(parsec completion zsh)
```

### Permanent Setup

```zsh
mkdir -p ~/.local/share/zsh/completions
parsec completion zsh > ~/.local/share/zsh/completions/_parsec
```
Then add the following to your `~/.zshrc` (before `compinit`):
```zsh
fpath=(~/.local/share/zsh/completions $fpath)
```

---

## PowerShell

### Load in Current Session
```powershell
parsec completion powershell | Out-String | Invoke-Expression
```

### Permanent Setup
To load completions for every new session, add the output of the above command to your PowerShell profile.
```powershell
parsec completion powershell >> $PROFILE
```
