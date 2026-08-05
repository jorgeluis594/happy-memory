#!/usr/bin/env bash

if [ -n "${ZSH_VERSION:-}" ]; then
  case ${ZSH_EVAL_CONTEXT:-} in
    *:file) ;;
    *)
      printf '%s\n' 'Error: run this script with: source scripts/create-worktree.sh' >&2
      exit 1
      ;;
  esac
elif [ -n "${BASH_VERSION:-}" ]; then
  if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    printf '%s\n' 'Error: run this script with: source scripts/create-worktree.sh' >&2
    exit 1
  fi
else
  printf '%s\n' 'Error: this script must be sourced from Bash or Zsh.' >&2
  exit 1
fi

_happy_memory_create_worktree() {
  local repository_root branch_name directory_name worktree_path

  repository_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
    printf '%s\n' 'Error: the current directory is not inside a Git repository.' >&2
    return 1
  }

  printf '%s' 'New branch name: '
  IFS= read -r branch_name || {
    printf '\n%s\n' 'Error: unable to read a branch name.' >&2
    return 1
  }

  if [ -z "$branch_name" ]; then
    printf '%s\n' 'Error: the branch name cannot be empty.' >&2
    return 1
  fi

  if ! git check-ref-format --branch "$branch_name" >/dev/null 2>&1; then
    printf "Error: '%s' is not a valid branch name.\n" "$branch_name" >&2
    return 1
  fi

  if git -C "$repository_root" show-ref --verify --quiet "refs/heads/$branch_name"; then
    printf "Error: branch '%s' already exists.\n" "$branch_name" >&2
    return 1
  fi

  directory_name=$(printf '%s' "$branch_name" | tr '/' '-')
  worktree_path="$(dirname "$repository_root")/happy-memory-$directory_name"

  if [ -e "$worktree_path" ] || [ -L "$worktree_path" ]; then
    printf "Error: path '%s' already exists.\n" "$worktree_path" >&2
    return 1
  fi

  if ! git -C "$repository_root" worktree add -b "$branch_name" "$worktree_path" HEAD; then
    printf '%s\n' 'Error: unable to create the worktree.' >&2
    return 1
  fi

  if ! cd "$worktree_path"; then
    printf "Error: worktree created, but could not change to '%s'.\n" "$worktree_path" >&2
    return 1
  fi

  printf "Created worktree for branch '%s' at '%s'.\n" "$branch_name" "$worktree_path"
}

if _happy_memory_create_worktree; then
  unset -f _happy_memory_create_worktree
  return 0
else
  unset -f _happy_memory_create_worktree
  return 1
fi
