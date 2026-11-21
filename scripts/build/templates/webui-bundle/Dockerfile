FROM scratch

WORKDIR /app
COPY cfst-webui /app/cfst-webui
COPY LICENSE /app/LICENSE
COPY THIRD-PARTY-NOTICES.md /app/THIRD-PARTY-NOTICES.md
COPY SOURCE.txt /app/SOURCE.txt
COPY ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY CA-CERTIFICATES-COPYRIGHT /app/CA-CERTIFICATES-COPYRIGHT
EXPOSE 34115
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/app/cfst-webui", "--healthcheck"]
ENTRYPOINT ["/app/cfst-webui"]
