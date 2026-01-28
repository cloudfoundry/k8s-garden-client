FROM mcr.microsoft.com/vscode/devcontainers/go:1

RUN mkdir -p /home/vscode/.local/bin
RUN arch="$(go env GOARCH)" \
    && if [ "$arch" = "arm64" ]; then file_arch="386"; else file_arch="$arch"; fi \
    && wget -q "https://github.com/git-duet/git-duet/releases/download/0.10.0/linux_${file_arch}.tar.gz" -O - \
    | tar -C /home/vscode/.local/bin -xzf -
