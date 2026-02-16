FROM mcr.microsoft.com/vscode/devcontainers/go:1

# Fix Yarn repository GPG key issue
RUN curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | gpg --dearmor | tee /usr/share/keyrings/yarn-archive-keyring.gpg >/dev/null \
    && echo "deb [signed-by=/usr/share/keyrings/yarn-archive-keyring.gpg] https://dl.yarnpkg.com/debian stable main" | tee /etc/apt/sources.list.d/yarn.list \
    && apt-get update

RUN mkdir -p /home/vscode/.local/bin
RUN arch="$(go env GOARCH)" \
    && if [ "$arch" = "arm64" ]; then file_arch="386"; else file_arch="$arch"; fi \
    && wget -q "https://github.com/git-duet/git-duet/releases/download/0.10.0/linux_${file_arch}.tar.gz" -O - \
    | tar -C /home/vscode/.local/bin -xzf -
