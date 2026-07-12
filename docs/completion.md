# Shell Completion

`parsec` supports generating autocompletion scripts for various shells. This allows you to use the `Tab` key to complete commands, flags, and arguments.

## Preset Completion

The `--preset` / `-p` flag supports dynamic completion. When you press `Tab` after `--preset`, parsec reads your active configuration file and suggests the preset names defined in it.

```
parsec rename --preset <Tab>
anime        encode       mandalorian  movie-remux  quick-check
```

In shells that support completion descriptions (zsh, fish, PowerShell), the optional [`description`](config.md#defining-presets) field from each preset is shown as a hint:

```
parsec rename --preset <Tab>
anime        -- Anime naming scheme with CRC32 and no word separator
encode       -- BluRay encode using x264/x265 codec labels
mandalorian  -- Pin metadata to The Mandalorian on DSNP
movie-remux  -- High-quality BluRay remux (HEVC/AVC codec labels)
```

> **Note:** Bash does not display completion descriptions — preset names still complete correctly, descriptions are simply not shown.

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
