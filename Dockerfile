FROM alpine:3.20
COPY grade.sh /grade.sh
RUN chmod +x /grade.sh
ENTRYPOINT ["/grade.sh"]
