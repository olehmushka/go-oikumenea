export interface IConnectorConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Connector:ConnectorConflict";
    'parameters': {
        reason: string;
    };
}

export function isConnectorConflict(arg: any): arg is IConnectorConflict {
    return arg && arg.errorName === "Connector:ConnectorConflict";
}
