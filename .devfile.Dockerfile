FROM quay.io/devfile/universal-developer-image:ubi8-latest

#install Go 1.19
RUN cd /tmp && wget https://go.dev/dl/go1.19.13.linux-amd64.tar.gz && \
        mkdir $HOME/go1.19 && \
        tar -xvzf go1.19.13.linux-amd64.tar.gz -C $HOME/go1.19 --strip-components 1 && \
        if ! grep -q "export PATH=\$HOME/go1.19/bin:\$PATH" $HOME/.bashrc; then echo "export PATH=\$HOME/go1.19/bin:\$PATH" >> $HOME/.bashrc; fi

# install chectl
RUN tag=$(curl https://api.github.com/repos/che-incubator/chectl/tags | jq -r '.[0].name') && \
        cd /tmp && wget https://github.com/che-incubator/chectl/releases/download/${tag}/chectl-linux-x64.tar.gz && \
        mkdir $HOME/chectl && \
        tar -xvzf chectl-linux-x64.tar.gz -C $HOME/chectl --strip-components 1 && \
        if ! grep -q "export PATH=\$HOME/chectl/bin:\$PATH" $HOME/.bashrc; then echo "export PATH=\$HOME/chectl/bin:\$PATH" >> $HOME/.bashrc; fi

# install goimports
RUN $HOME/go1.19/bin/go install golang.org/x/tools/cmd/goimports@latest
