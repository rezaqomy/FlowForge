package main

const fishCompletion = `function __flowforge_resource_names
    set -l tokens (commandline -opc)
    set -l executable $tokens[1]
    set -l kind $tokens[3]
    switch $kind
        case secret secrets
            set kind secrets
        case workflow workflows
            set kind workflows
        case '*'
            return
    end
    command $executable get $kind 2>/dev/null | string match -rg '"name":\s*"([^"]+)"'
end

complete -c flowforge -f
complete -c flowforge -n '__fish_use_subcommand' -a apply -d 'Create or update a resource'
complete -c flowforge -n '__fish_use_subcommand' -a get -d 'Get one resource or list resources'
complete -c flowforge -n '__fish_use_subcommand' -a delete -d 'Delete a resource'
complete -c flowforge -n '__fish_use_subcommand' -a completion -d 'Generate shell completion'
complete -c flowforge -n '__fish_use_subcommand' -a help -d 'Show usage'
complete -c flowforge -n '__fish_seen_subcommand_from apply' -s f -d 'Resource manifest' -r -F
complete -c flowforge -n '__fish_seen_subcommand_from apply' -l server -d 'Server base URL' -r
complete -c flowforge -n '__fish_seen_subcommand_from get; and not __fish_seen_subcommand_from secret secrets workflow workflows' -a 'secret secrets workflow workflows' -d 'Resource kind'
complete -c flowforge -n '__fish_seen_subcommand_from get' -s o -l output -d 'Output format' -xa 'json yaml'
complete -c flowforge -n '__fish_seen_subcommand_from get' -l server -d 'Server base URL' -r
complete -c flowforge -n '__fish_seen_subcommand_from get; and __fish_seen_subcommand_from secret workflow' -a '(__flowforge_resource_names)' -d 'Resource name'
complete -c flowforge -n '__fish_seen_subcommand_from delete; and not __fish_seen_subcommand_from secret secrets workflow workflows' -a 'secret workflow' -d 'Resource kind'
complete -c flowforge -n '__fish_seen_subcommand_from delete' -s f -d 'Resource manifest' -r -F
complete -c flowforge -n '__fish_seen_subcommand_from delete' -l server -d 'Server base URL' -r
complete -c flowforge -n '__fish_seen_subcommand_from delete; and __fish_seen_subcommand_from secret workflow' -a '(__flowforge_resource_names)' -d 'Resource name'
complete -c flowforge -n '__fish_seen_subcommand_from delete; and __fish_seen_subcommand_from secret workflow' -F
complete -c flowforge -n '__fish_seen_subcommand_from completion' -a 'fish bash zsh install' -d 'Shell or action'
complete -c flowforge -n '__fish_seen_subcommand_from completion; and __fish_seen_subcommand_from install' -a fish -d 'Shell'
`

const bashCompletion = `_flowforge_completion() {
    local current previous
    current="${COMP_WORDS[COMP_CWORD]}"
    previous="${COMP_WORDS[COMP_CWORD-1]}"

    if [[ $COMP_CWORD -eq 1 ]]; then
        COMPREPLY=($(compgen -W "apply get delete completion help" -- "$current"))
        return
    fi

    case "${COMP_WORDS[1]}" in
        apply)
            if [[ "$previous" == "-f" ]]; then
                COMPREPLY=($(compgen -f -- "$current"))
            else
                COMPREPLY=($(compgen -W "-f --server" -- "$current"))
            fi
            ;;
        get)
            if [[ $COMP_CWORD -eq 2 ]]; then
                COMPREPLY=($(compgen -W "secret secrets workflow workflows" -- "$current"))
            elif [[ "$previous" == "-o" || "$previous" == "--output" ]]; then
                COMPREPLY=($(compgen -W "json yaml" -- "$current"))
            else
                COMPREPLY=($(compgen -W "-o --output --server" -- "$current"))
            fi
            ;;
        delete)
            if [[ $COMP_CWORD -eq 2 ]]; then
                COMPREPLY=($(compgen -W "secret workflow -f --server" -- "$current"))
            elif [[ "$previous" == "-f" || "$previous" == "secret" || "$previous" == "workflow" ]]; then
                COMPREPLY=($(compgen -f -- "$current"))
            else
                COMPREPLY=($(compgen -W "--server" -- "$current"))
            fi
            ;;
        completion)
        COMPREPLY=($(compgen -W "fish bash zsh install" -- "$current"))
            ;;
    esac
}
complete -F _flowforge_completion flowforge
`

const zshCompletion = `#compdef flowforge

_flowforge() {
  local -a commands
  commands=(
    'apply:Create or update a resource'
    'get:Get one resource or list resources'
    'delete:Delete a resource'
    'completion:Generate shell completion'
    'help:Show usage'
  )

  if (( CURRENT == 2 )); then
    _describe 'command' commands
    return
  fi

  case "$words[2]" in
    apply)
      _arguments '-f[Resource manifest]:manifest:_files' '--server[Server base URL]:url:'
      ;;
    get)
      _arguments \
        '1:resource kind:(secret secrets workflow workflows)' \
        '2:resource name:' \
        '(-o --output)'{-o,--output}'[Output format]:format:(json yaml)' \
        '--server[Server base URL]:url:'
      ;;
    delete)
      _arguments \
        '1:resource kind:(secret workflow)' \
        '2:resource name or manifest:_files' \
        '-f[Resource manifest]:manifest:_files' \
        '--server[Server base URL]:url:'
      ;;
    completion)
      _values 'shell or action' fish bash zsh install
      ;;
  esac
}

compdef _flowforge flowforge
`
