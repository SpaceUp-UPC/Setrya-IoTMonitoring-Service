-- Contrato del Edge: ids acordados (hasta 64 caracteres), campos extra de las
-- lecturas y datos de evento en las alertas.
ALTER TABLE iot_alerts DROP FOREIGN KEY fk_alerts_reading, DROP FOREIGN KEY fk_alerts_device;

ALTER TABLE iot_readings DROP FOREIGN KEY fk_readings_device;

ALTER TABLE iot_devices MODIFY id VARCHAR(64) NOT NULL;

ALTER TABLE iot_readings
    MODIFY id VARCHAR(64) NOT NULL,
    MODIFY device_id VARCHAR(64) NOT NULL,
    ADD COLUMN measured_at DATETIME(6) NULL,
    ADD COLUMN local_state VARCHAR(16) NOT NULL DEFAULT '';

ALTER TABLE iot_alerts
    MODIFY id VARCHAR(64) NOT NULL,
    MODIFY reading_id VARCHAR(64) NOT NULL,
    MODIFY device_id VARCHAR(64) NOT NULL,
    ADD COLUMN metric_type VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN reason VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN event_id VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN evidence_ref VARCHAR(255) NOT NULL DEFAULT '',
    ADD INDEX idx_alerts_device_open (device_id, metric_type, status),
    ADD INDEX idx_alerts_reading (reading_id);

ALTER TABLE iot_readings
    ADD CONSTRAINT fk_readings_device FOREIGN KEY (device_id) REFERENCES iot_devices (id);

ALTER TABLE iot_alerts
    ADD CONSTRAINT fk_alerts_reading FOREIGN KEY (reading_id) REFERENCES iot_readings (id),
    ADD CONSTRAINT fk_alerts_device FOREIGN KEY (device_id) REFERENCES iot_devices (id);
