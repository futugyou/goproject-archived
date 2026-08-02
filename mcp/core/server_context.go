package core

// type  MessageContext struct{
// Server McpSession
// }

//     public MessageContext(McpServer server, JsonRpcMessage jsonRpcMessage)
//     {
//         Throw.IfNull(server);
//         Throw.IfNull(jsonRpcMessage);

//         Server = server;
//         JsonRpcMessage = jsonRpcMessage;
//         Services = server.Services;
//     }

//     /// <summary>Gets or sets the server with which this instance is associated.</summary>
//     public McpServer Server
//     {
//         get => field;
//         set
//         {
//             Throw.IfNull(value);
//             field = value;
//         }
//     }

//     /// <summary>
//     /// Gets or sets a key/value collection that can be used to share data within the scope of this message.
//     /// </summary>
//     /// <remarks>
//     /// <para>
//     /// This dictionary is shared with the <see cref="Protocol.JsonRpcMessageContext.Items"/> property
//     /// on the underlying <see cref="JsonRpcMessage"/>, ensuring that data set in message filters
//     /// flows through to request-specific filters and handlers.
//     /// </para>
//     /// </remarks>
//     public IDictionary<string, object?> Items
//     {
//         get
//         {
//             JsonRpcMessage.Context ??= new();
//             return JsonRpcMessage.Context.Items ??= new Dictionary<string, object?>();
//         }
//         set
//         {
//             JsonRpcMessage.Context ??= new();
//             JsonRpcMessage.Context.Items = value;
//         }
//     }

//     public JsonRpcMessage JsonRpcMessage { get; set; }
// }

// type RequestContext[TParams any] struct {
// 	Params TParams
// 	Server IMcpServer
// }
