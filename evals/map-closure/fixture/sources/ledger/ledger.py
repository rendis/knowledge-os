TOPIC = "shipment-ready"
TABLE = "shipment_ledger"


def register(subscriber, database):
    subscriber.subscribe(TOPIC, lambda event: database.insert(TABLE, event))
