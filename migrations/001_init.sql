CREATE TABLE IF NOT EXISTS iot_devices (
    id         CHAR(36)     NOT NULL PRIMARY KEY,
    space_id   VARCHAR(64)  NOT NULL,
    name       VARCHAR(120) NOT NULL,
    active     BOOLEAN      NOT NULL DEFAULT FALSE,
    thresholds JSON         NOT NULL,
    created_at DATETIME(6)  NOT NULL,
    INDEX idx_devices_space (space_id)
);

CREATE TABLE IF NOT EXISTS iot_readings (
    id          CHAR(36)    NOT NULL PRIMARY KEY,
    device_id   CHAR(36)    NOT NULL,
    metric_type VARCHAR(32) NOT NULL,
    value       DOUBLE      NOT NULL,
    unit        VARCHAR(16) NOT NULL,
    status      VARCHAR(16) NOT NULL,
    received_at DATETIME(6) NOT NULL,
    INDEX idx_readings_device_received (device_id, received_at),
    CONSTRAINT fk_readings_device FOREIGN KEY (device_id) REFERENCES iot_devices (id)
);

CREATE TABLE IF NOT EXISTS iot_alerts (
    id              CHAR(36)     NOT NULL PRIMARY KEY,
    reading_id      CHAR(36)     NOT NULL,
    device_id       CHAR(36)     NOT NULL,
    space_id        VARCHAR(64)  NOT NULL,
    severity        VARCHAR(16)  NOT NULL,
    status          VARCHAR(16)  NOT NULL,
    message         VARCHAR(255) NOT NULL,
    created_at      DATETIME(6)  NOT NULL,
    acknowledged_at DATETIME(6)  NULL,
    closed_at       DATETIME(6)  NULL,
    INDEX idx_alerts_space_status (space_id, status, created_at),
    CONSTRAINT fk_alerts_reading FOREIGN KEY (reading_id) REFERENCES iot_readings (id),
    CONSTRAINT fk_alerts_device FOREIGN KEY (device_id) REFERENCES iot_devices (id)
);
