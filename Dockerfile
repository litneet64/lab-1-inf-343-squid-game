FROM golang:1.17

ENV PATH="${PATH}:/root/.local/bin:/go/bin"
ENV PB_URL="https://github.com/protocolbuffers/protobuf/releases/download"
ARG PROTO_VER="v3.19.0"
ARG ZIP_NAME="protoc-3.19.0-linux-x86_64.zip"

WORKDIR "/go/src/lab-2-squid-game"

RUN apt update && apt install unzip -y \
    && curl -LO $PB_URL/$PROTO_VER/$ZIP_NAME \
    && unzip $ZIP_NAME -d $HOME/.local \
    && go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.26 \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1 

COPY . ./

RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    $(find ./ -name "*.proto")

RUN for dir in leader namenode datanode player pool; do \
    go build -o $dir/$dir -v $dir/main.go; \
    done

CMD ["-c", "cat go.mod && echo -------- && cat go.sum"]
ENTRYPOINT ["/bin/bash"]
