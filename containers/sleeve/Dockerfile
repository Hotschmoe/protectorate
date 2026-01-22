ARG SLEEVE_BASE_TAG=latest
FROM protectorate/sleeve-base:${SLEEVE_BASE_TAG}

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
