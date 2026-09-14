TOPIC = "shipment-ready"


def publish_shipment(publisher, shipment_id):
    publisher.publish(TOPIC, {"shipment_id": shipment_id})
