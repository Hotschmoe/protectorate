ARG BASE_IMAGE=protectorate/base
ARG BASE_TAG=latest
FROM ${BASE_IMAGE}:${BASE_TAG}

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
