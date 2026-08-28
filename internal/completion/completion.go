package completion

import (
	"fmt"
	"io"
)

// GenerateCompletion writes the shell completion script to the given writer.
func GenerateCompletion(shell string, w io.Writer) error {
	switch shell {
	case "bash":
		_, err := fmt.Fprint(w, bashCompletion)
		return err
	case "zsh":
		_, err := fmt.Fprint(w, zshCompletion)
		return err
	case "fish":
		_, err := fmt.Fprint(w, fishCompletion)
		return err
	case "powershell", "ps":
		_, err := fmt.Fprint(w, powershellCompletion)
		return err
	default:
		return fmt.Errorf("unsupported shell '%s'. Supported shells: bash, zsh, fish, powershell", shell)
	}
}

const bashCompletion = `# bash completion for killport
_killport_complete() {
    local cur prev words cword
    _init_completion || return

    local flags="-f --force -g --graceful -t --timeout -p --protocol -d --dry-run -l --list -i --interactive -j --json -v --verbose -q --quiet -w --watch -W --wait -T --tree --inspect --no-color --version -h --help completion"
    local presets="web dev db database next nextjs react remix vite astro svelte vue angular django fastapi flask rails redis postgres postgresql mysql mariadb mongo mongodb"

    if [[ "$cur" == -* ]]; then
        COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
        return 0
    fi

    if [[ "$prev" == "completion" ]]; then
        COMPREPLY=( $(compgen -W "bash zsh fish powershell" -- "$cur") )
        return 0
    fi

    if [[ "$prev" == "-p" || "$prev" == "--protocol" ]]; then
        COMPREPLY=( $(compgen -W "tcp udp all" -- "$cur") )
        return 0
    fi

    # Suggest active listening ports and presets
    local ports
    if command -v killport >/dev/null 2>&1; then
        ports=$(killport -l --no-color 2>/dev/null | awk 'NR>2 {print $1}' | sort -u)
    fi

    COMPREPLY=( $(compgen -W "$ports $presets" -- "$cur") )
}

complete -F _killport_complete killport kport
`

const zshCompletion = `#compdef killport kport

_killport() {
    local -a flags presets ports

    flags=(
        '(-f --force)'{-f,--force}'[Force kill immediately (SIGKILL)]'
        '(-g --graceful)'{-g,--graceful}'[Graceful termination (SIGTERM first)]'
        '(-t --timeout)'{-t,--timeout}'[Graceful shutdown timeout]:duration:'
        '(-p --protocol)'{-p,--protocol}'[Protocol filter (tcp, udp, all)]:protocol:(tcp udp all)'
        '(-d --dry-run)'{-d,--dry-run}'[Preview processes without killing]'
        '(-l --list)'{-l,--list}'[List listening processes]'
        '(-i --interactive)'{-i,--interactive}'[Prompt before killing each process]'
        '(-j --json)'{-j,--json}'[Output results in JSON format]'
        '(-v --verbose)'{-v,--verbose}'[Show full command lines and details]'
        '(-q --quiet)'{-q,--quiet}'[Quiet mode]'
        '(-w --watch)'{-w,--watch}'[Watch and continuously kill processes on ports]'
        '(-W --wait)'{-W,--wait}'[Wait until port is completely released before exit]'
        '(-T --tree)'{-T,--tree}'[Kill process tree (descendant children)]'
        '--inspect[Show detailed memory, CWD, and tree info]'
        '--no-color[Disable colored output]'
        '(-h --help)'{-h,--help}'[Show help message]'
        '--version[Print version]'
    )

    presets=(
        'web:Standard web dev ports (3000, 5173, 8080, ...)'
        'dev:Development ports'
        'db:Database ports (5432, 3306, 27017, 6379, ...)'
        'next:Next.js (3000)'
        'react:React (3000)'
        'vite:Vite (5173)'
        'astro:Astro (4321)'
        'svelte:SvelteKit (5173)'
        'vue:Vue.js (8080)'
        'angular:Angular (4200)'
        'django:Django (8000)'
        'fastapi:FastAPI (8000)'
        'flask:Flask (5000)'
        'rails:Ruby on Rails (3000)'
        'redis:Redis (6379)'
        'postgres:PostgreSQL (5432)'
        'mysql:MySQL (3306)'
        'mongo:MongoDB (27017)'
    )

    _arguments -s \
        $flags \
        '1: :->cmd_or_port' \
        '*: :->extra_ports'

    case $state in
        cmd_or_port)
            if [[ $words[CURRENT] != -* ]]; then
                _describe 'presets' presets
                _message 'port or port range (e.g. 3000, :8080, 3000-3005)'
            fi
            ;;
        extra_ports)
            _describe 'presets' presets
            _message 'port'
            ;;
    esac
}

_killport "$@"
`

const fishCompletion = `# fish completion for killport
complete -c killport -s f -l force -d "Force kill immediately (SIGKILL)"
complete -c killport -s g -l graceful -d "Graceful termination"
complete -c killport -s t -l timeout -d "Graceful shutdown timeout" -r
complete -c killport -s p -l protocol -d "Protocol filter" -a "tcp udp all"
complete -c killport -s d -l dry-run -d "Preview processes without killing"
complete -c killport -s l -l list -d "List listening processes"
complete -c killport -s i -l interactive -d "Interactive confirmation"
complete -c killport -s j -l json -d "JSON output"
complete -c killport -s v -l verbose -d "Verbose output"
complete -c killport -s q -l quiet -d "Quiet mode"
complete -c killport -s w -l watch -d "Watch mode"
complete -c killport -s W -l wait -d "Wait for release"
complete -c killport -s T -l tree -d "Kill process tree"
complete -c killport -l inspect -d "Inspect process details"
complete -c killport -l no-color -d "Disable color output"
complete -c killport -s h -l help -d "Show help"
complete -c killport -l version -d "Print version"

complete -c kport -w killport
`

const powershellCompletion = `Register-ArgumentCompleter -Native -CommandName killport,kport -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $options = @(
        '-f', '--force', '-g', '--graceful', '-t', '--timeout',
        '-p', '--protocol', '-d', '--dry-run', '-l', '--list',
        '-i', '--interactive', '-j', '--json', '-v', '--verbose',
        '-q', '--quiet', '-w', '--watch', '-W', '--wait', '-T', '--tree',
        '--inspect', '--no-color', '--version', '-h', '--help',
        'web', 'dev', 'db', 'next', 'react', 'vite', 'redis', 'postgres', 'mysql'
    )
    $options | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`
