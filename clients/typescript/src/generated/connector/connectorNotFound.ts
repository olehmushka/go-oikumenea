export interface IConnectorNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Connector:ConnectorNotFound";
    'parameters': {
        connectorId: string;
    };
}

export function isConnectorNotFound(arg: any): arg is IConnectorNotFound {
    return arg && arg.errorName === "Connector:ConnectorNotFound";
}
