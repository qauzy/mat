FROM alpine:latest as builder
ARG TARGETPLATFORM
RUN echo "I'm building for $TARGETPLATFORM"

RUN apk add --no-cache gzip && \
    mkdir /mat-config && \
    curl     -x http://127.0.0.1:7890 -L -o  /mat-config/geoip.metadb https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb && \
    curl     -x http://127.0.0.1:7890 -L -o  /mat-config/geosite.dat https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat && \
    curl     -x http://127.0.0.1:7890 -L -o  /mat-config/geoip.dat https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.dat

COPY docker/file-name.sh /mat/file-name.sh
WORKDIR /mat
COPY bin/ bin/
RUN FILE_NAME=`sh file-name.sh` && echo $FILE_NAME && \
    FILE_NAME=`ls bin/ | egrep "$FILE_NAME.gz"|awk NR==1` && echo $FILE_NAME && \
    mv bin/$FILE_NAME mat.gz && gzip -d mat.gz && echo "$FILE_NAME" > /mat-config/test
FROM alpine:latest
LABEL org.opencontainers.image.source="https://github.com/MetaCubeX/mat"

RUN apk add --no-cache ca-certificates tzdata iptables

VOLUME ["/root/.config/mat/"]

COPY --from=builder /mat-config/ /root/.config/mat/
COPY --from=builder /mat/mat /mat
RUN chmod +x /mat
ENTRYPOINT [ "/mat" ]
