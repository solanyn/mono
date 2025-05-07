/*eslint-disable block-scoped-var, id-length, no-control-regex, no-magic-numbers, no-prototype-builtins, no-redeclare, no-shadow, no-var, sort-vars*/
import * as $protobuf from "protobufjs/minimal";

// Common aliases
const $Reader = $protobuf.Reader, $Writer = $protobuf.Writer, $util = $protobuf.util;

// Exported root namespace
const $root = $protobuf.roots["default"] || ($protobuf.roots["default"] = {});

export const tldr = $root.tldr = (() => {

    /**
     * Namespace tldr.
     * @exports tldr
     * @namespace
     */
    const tldr = {};

    tldr.news = (function() {

        /**
         * Namespace news.
         * @memberof tldr
         * @namespace
         */
        const news = {};

        news.v1 = (function() {

            /**
             * Namespace v1.
             * @memberof tldr.news
             * @namespace
             */
            const v1 = {};

            v1.NewsSummary = (function() {

                /**
                 * Properties of a NewsSummary.
                 * @memberof tldr.news.v1
                 * @interface INewsSummary
                 * @property {string|null} [date] NewsSummary date
                 */

                /**
                 * Constructs a new NewsSummary.
                 * @memberof tldr.news.v1
                 * @classdesc Represents a NewsSummary.
                 * @implements INewsSummary
                 * @constructor
                 * @param {tldr.news.v1.INewsSummary=} [properties] Properties to set
                 */
                function NewsSummary(properties) {
                    if (properties)
                        for (let keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * NewsSummary date.
                 * @member {string} date
                 * @memberof tldr.news.v1.NewsSummary
                 * @instance
                 */
                NewsSummary.prototype.date = "";

                /**
                 * Creates a new NewsSummary instance using the specified properties.
                 * @function create
                 * @memberof tldr.news.v1.NewsSummary
                 * @static
                 * @param {tldr.news.v1.INewsSummary=} [properties] Properties to set
                 * @returns {tldr.news.v1.NewsSummary} NewsSummary instance
                 */
                NewsSummary.create = function create(properties) {
                    return new NewsSummary(properties);
                };

                /**
                 * Encodes the specified NewsSummary message. Does not implicitly {@link tldr.news.v1.NewsSummary.verify|verify} messages.
                 * @function encode
                 * @memberof tldr.news.v1.NewsSummary
                 * @static
                 * @param {tldr.news.v1.INewsSummary} message NewsSummary message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                NewsSummary.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.date != null && Object.hasOwnProperty.call(message, "date"))
                        writer.uint32(/* id 1, wireType 2 =*/10).string(message.date);
                    return writer;
                };

                /**
                 * Encodes the specified NewsSummary message, length delimited. Does not implicitly {@link tldr.news.v1.NewsSummary.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof tldr.news.v1.NewsSummary
                 * @static
                 * @param {tldr.news.v1.INewsSummary} message NewsSummary message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                NewsSummary.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a NewsSummary message from the specified reader or buffer.
                 * @function decode
                 * @memberof tldr.news.v1.NewsSummary
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {tldr.news.v1.NewsSummary} NewsSummary
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                NewsSummary.decode = function decode(reader, length) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    let end = length === undefined ? reader.len : reader.pos + length, message = new $root.tldr.news.v1.NewsSummary();
                    while (reader.pos < end) {
                        let tag = reader.uint32();
                        switch (tag >>> 3) {
                        case 1: {
                                message.date = reader.string();
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a NewsSummary message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof tldr.news.v1.NewsSummary
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {tldr.news.v1.NewsSummary} NewsSummary
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                NewsSummary.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a NewsSummary message.
                 * @function verify
                 * @memberof tldr.news.v1.NewsSummary
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                NewsSummary.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.date != null && message.hasOwnProperty("date"))
                        if (!$util.isString(message.date))
                            return "date: string expected";
                    return null;
                };

                /**
                 * Creates a NewsSummary message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof tldr.news.v1.NewsSummary
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {tldr.news.v1.NewsSummary} NewsSummary
                 */
                NewsSummary.fromObject = function fromObject(object) {
                    if (object instanceof $root.tldr.news.v1.NewsSummary)
                        return object;
                    let message = new $root.tldr.news.v1.NewsSummary();
                    if (object.date != null)
                        message.date = String(object.date);
                    return message;
                };

                /**
                 * Creates a plain object from a NewsSummary message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof tldr.news.v1.NewsSummary
                 * @static
                 * @param {tldr.news.v1.NewsSummary} message NewsSummary
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                NewsSummary.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    let object = {};
                    if (options.defaults)
                        object.date = "";
                    if (message.date != null && message.hasOwnProperty("date"))
                        object.date = message.date;
                    return object;
                };

                /**
                 * Converts this NewsSummary to JSON.
                 * @function toJSON
                 * @memberof tldr.news.v1.NewsSummary
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                NewsSummary.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for NewsSummary
                 * @function getTypeUrl
                 * @memberof tldr.news.v1.NewsSummary
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                NewsSummary.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/tldr.news.v1.NewsSummary";
                };

                return NewsSummary;
            })();

            v1.ListNewsSummariesResponse = (function() {

                /**
                 * Properties of a ListNewsSummariesResponse.
                 * @memberof tldr.news.v1
                 * @interface IListNewsSummariesResponse
                 * @property {Array.<tldr.news.v1.INewsSummary>|null} [summaries] ListNewsSummariesResponse summaries
                 */

                /**
                 * Constructs a new ListNewsSummariesResponse.
                 * @memberof tldr.news.v1
                 * @classdesc Represents a ListNewsSummariesResponse.
                 * @implements IListNewsSummariesResponse
                 * @constructor
                 * @param {tldr.news.v1.IListNewsSummariesResponse=} [properties] Properties to set
                 */
                function ListNewsSummariesResponse(properties) {
                    this.summaries = [];
                    if (properties)
                        for (let keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * ListNewsSummariesResponse summaries.
                 * @member {Array.<tldr.news.v1.INewsSummary>} summaries
                 * @memberof tldr.news.v1.ListNewsSummariesResponse
                 * @instance
                 */
                ListNewsSummariesResponse.prototype.summaries = $util.emptyArray;

                /**
                 * Creates a new ListNewsSummariesResponse instance using the specified properties.
                 * @function create
                 * @memberof tldr.news.v1.ListNewsSummariesResponse
                 * @static
                 * @param {tldr.news.v1.IListNewsSummariesResponse=} [properties] Properties to set
                 * @returns {tldr.news.v1.ListNewsSummariesResponse} ListNewsSummariesResponse instance
                 */
                ListNewsSummariesResponse.create = function create(properties) {
                    return new ListNewsSummariesResponse(properties);
                };

                /**
                 * Encodes the specified ListNewsSummariesResponse message. Does not implicitly {@link tldr.news.v1.ListNewsSummariesResponse.verify|verify} messages.
                 * @function encode
                 * @memberof tldr.news.v1.ListNewsSummariesResponse
                 * @static
                 * @param {tldr.news.v1.IListNewsSummariesResponse} message ListNewsSummariesResponse message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                ListNewsSummariesResponse.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.summaries != null && message.summaries.length)
                        for (let i = 0; i < message.summaries.length; ++i)
                            $root.tldr.news.v1.NewsSummary.encode(message.summaries[i], writer.uint32(/* id 1, wireType 2 =*/10).fork()).ldelim();
                    return writer;
                };

                /**
                 * Encodes the specified ListNewsSummariesResponse message, length delimited. Does not implicitly {@link tldr.news.v1.ListNewsSummariesResponse.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof tldr.news.v1.ListNewsSummariesResponse
                 * @static
                 * @param {tldr.news.v1.IListNewsSummariesResponse} message ListNewsSummariesResponse message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                ListNewsSummariesResponse.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a ListNewsSummariesResponse message from the specified reader or buffer.
                 * @function decode
                 * @memberof tldr.news.v1.ListNewsSummariesResponse
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {tldr.news.v1.ListNewsSummariesResponse} ListNewsSummariesResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                ListNewsSummariesResponse.decode = function decode(reader, length) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    let end = length === undefined ? reader.len : reader.pos + length, message = new $root.tldr.news.v1.ListNewsSummariesResponse();
                    while (reader.pos < end) {
                        let tag = reader.uint32();
                        switch (tag >>> 3) {
                        case 1: {
                                if (!(message.summaries && message.summaries.length))
                                    message.summaries = [];
                                message.summaries.push($root.tldr.news.v1.NewsSummary.decode(reader, reader.uint32()));
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a ListNewsSummariesResponse message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof tldr.news.v1.ListNewsSummariesResponse
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {tldr.news.v1.ListNewsSummariesResponse} ListNewsSummariesResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                ListNewsSummariesResponse.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a ListNewsSummariesResponse message.
                 * @function verify
                 * @memberof tldr.news.v1.ListNewsSummariesResponse
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                ListNewsSummariesResponse.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.summaries != null && message.hasOwnProperty("summaries")) {
                        if (!Array.isArray(message.summaries))
                            return "summaries: array expected";
                        for (let i = 0; i < message.summaries.length; ++i) {
                            let error = $root.tldr.news.v1.NewsSummary.verify(message.summaries[i]);
                            if (error)
                                return "summaries." + error;
                        }
                    }
                    return null;
                };

                /**
                 * Creates a ListNewsSummariesResponse message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof tldr.news.v1.ListNewsSummariesResponse
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {tldr.news.v1.ListNewsSummariesResponse} ListNewsSummariesResponse
                 */
                ListNewsSummariesResponse.fromObject = function fromObject(object) {
                    if (object instanceof $root.tldr.news.v1.ListNewsSummariesResponse)
                        return object;
                    let message = new $root.tldr.news.v1.ListNewsSummariesResponse();
                    if (object.summaries) {
                        if (!Array.isArray(object.summaries))
                            throw TypeError(".tldr.news.v1.ListNewsSummariesResponse.summaries: array expected");
                        message.summaries = [];
                        for (let i = 0; i < object.summaries.length; ++i) {
                            if (typeof object.summaries[i] !== "object")
                                throw TypeError(".tldr.news.v1.ListNewsSummariesResponse.summaries: object expected");
                            message.summaries[i] = $root.tldr.news.v1.NewsSummary.fromObject(object.summaries[i]);
                        }
                    }
                    return message;
                };

                /**
                 * Creates a plain object from a ListNewsSummariesResponse message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof tldr.news.v1.ListNewsSummariesResponse
                 * @static
                 * @param {tldr.news.v1.ListNewsSummariesResponse} message ListNewsSummariesResponse
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                ListNewsSummariesResponse.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    let object = {};
                    if (options.arrays || options.defaults)
                        object.summaries = [];
                    if (message.summaries && message.summaries.length) {
                        object.summaries = [];
                        for (let j = 0; j < message.summaries.length; ++j)
                            object.summaries[j] = $root.tldr.news.v1.NewsSummary.toObject(message.summaries[j], options);
                    }
                    return object;
                };

                /**
                 * Converts this ListNewsSummariesResponse to JSON.
                 * @function toJSON
                 * @memberof tldr.news.v1.ListNewsSummariesResponse
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                ListNewsSummariesResponse.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for ListNewsSummariesResponse
                 * @function getTypeUrl
                 * @memberof tldr.news.v1.ListNewsSummariesResponse
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                ListNewsSummariesResponse.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/tldr.news.v1.ListNewsSummariesResponse";
                };

                return ListNewsSummariesResponse;
            })();

            v1.GetNewsSummaryResponse = (function() {

                /**
                 * Properties of a GetNewsSummaryResponse.
                 * @memberof tldr.news.v1
                 * @interface IGetNewsSummaryResponse
                 * @property {string|null} [date] GetNewsSummaryResponse date
                 * @property {string|null} [content] GetNewsSummaryResponse content
                 */

                /**
                 * Constructs a new GetNewsSummaryResponse.
                 * @memberof tldr.news.v1
                 * @classdesc Represents a GetNewsSummaryResponse.
                 * @implements IGetNewsSummaryResponse
                 * @constructor
                 * @param {tldr.news.v1.IGetNewsSummaryResponse=} [properties] Properties to set
                 */
                function GetNewsSummaryResponse(properties) {
                    if (properties)
                        for (let keys = Object.keys(properties), i = 0; i < keys.length; ++i)
                            if (properties[keys[i]] != null)
                                this[keys[i]] = properties[keys[i]];
                }

                /**
                 * GetNewsSummaryResponse date.
                 * @member {string} date
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @instance
                 */
                GetNewsSummaryResponse.prototype.date = "";

                /**
                 * GetNewsSummaryResponse content.
                 * @member {string} content
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @instance
                 */
                GetNewsSummaryResponse.prototype.content = "";

                /**
                 * Creates a new GetNewsSummaryResponse instance using the specified properties.
                 * @function create
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @static
                 * @param {tldr.news.v1.IGetNewsSummaryResponse=} [properties] Properties to set
                 * @returns {tldr.news.v1.GetNewsSummaryResponse} GetNewsSummaryResponse instance
                 */
                GetNewsSummaryResponse.create = function create(properties) {
                    return new GetNewsSummaryResponse(properties);
                };

                /**
                 * Encodes the specified GetNewsSummaryResponse message. Does not implicitly {@link tldr.news.v1.GetNewsSummaryResponse.verify|verify} messages.
                 * @function encode
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @static
                 * @param {tldr.news.v1.IGetNewsSummaryResponse} message GetNewsSummaryResponse message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                GetNewsSummaryResponse.encode = function encode(message, writer) {
                    if (!writer)
                        writer = $Writer.create();
                    if (message.date != null && Object.hasOwnProperty.call(message, "date"))
                        writer.uint32(/* id 1, wireType 2 =*/10).string(message.date);
                    if (message.content != null && Object.hasOwnProperty.call(message, "content"))
                        writer.uint32(/* id 2, wireType 2 =*/18).string(message.content);
                    return writer;
                };

                /**
                 * Encodes the specified GetNewsSummaryResponse message, length delimited. Does not implicitly {@link tldr.news.v1.GetNewsSummaryResponse.verify|verify} messages.
                 * @function encodeDelimited
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @static
                 * @param {tldr.news.v1.IGetNewsSummaryResponse} message GetNewsSummaryResponse message or plain object to encode
                 * @param {$protobuf.Writer} [writer] Writer to encode to
                 * @returns {$protobuf.Writer} Writer
                 */
                GetNewsSummaryResponse.encodeDelimited = function encodeDelimited(message, writer) {
                    return this.encode(message, writer).ldelim();
                };

                /**
                 * Decodes a GetNewsSummaryResponse message from the specified reader or buffer.
                 * @function decode
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @param {number} [length] Message length if known beforehand
                 * @returns {tldr.news.v1.GetNewsSummaryResponse} GetNewsSummaryResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                GetNewsSummaryResponse.decode = function decode(reader, length) {
                    if (!(reader instanceof $Reader))
                        reader = $Reader.create(reader);
                    let end = length === undefined ? reader.len : reader.pos + length, message = new $root.tldr.news.v1.GetNewsSummaryResponse();
                    while (reader.pos < end) {
                        let tag = reader.uint32();
                        switch (tag >>> 3) {
                        case 1: {
                                message.date = reader.string();
                                break;
                            }
                        case 2: {
                                message.content = reader.string();
                                break;
                            }
                        default:
                            reader.skipType(tag & 7);
                            break;
                        }
                    }
                    return message;
                };

                /**
                 * Decodes a GetNewsSummaryResponse message from the specified reader or buffer, length delimited.
                 * @function decodeDelimited
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @static
                 * @param {$protobuf.Reader|Uint8Array} reader Reader or buffer to decode from
                 * @returns {tldr.news.v1.GetNewsSummaryResponse} GetNewsSummaryResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                GetNewsSummaryResponse.decodeDelimited = function decodeDelimited(reader) {
                    if (!(reader instanceof $Reader))
                        reader = new $Reader(reader);
                    return this.decode(reader, reader.uint32());
                };

                /**
                 * Verifies a GetNewsSummaryResponse message.
                 * @function verify
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @static
                 * @param {Object.<string,*>} message Plain object to verify
                 * @returns {string|null} `null` if valid, otherwise the reason why it is not
                 */
                GetNewsSummaryResponse.verify = function verify(message) {
                    if (typeof message !== "object" || message === null)
                        return "object expected";
                    if (message.date != null && message.hasOwnProperty("date"))
                        if (!$util.isString(message.date))
                            return "date: string expected";
                    if (message.content != null && message.hasOwnProperty("content"))
                        if (!$util.isString(message.content))
                            return "content: string expected";
                    return null;
                };

                /**
                 * Creates a GetNewsSummaryResponse message from a plain object. Also converts values to their respective internal types.
                 * @function fromObject
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @static
                 * @param {Object.<string,*>} object Plain object
                 * @returns {tldr.news.v1.GetNewsSummaryResponse} GetNewsSummaryResponse
                 */
                GetNewsSummaryResponse.fromObject = function fromObject(object) {
                    if (object instanceof $root.tldr.news.v1.GetNewsSummaryResponse)
                        return object;
                    let message = new $root.tldr.news.v1.GetNewsSummaryResponse();
                    if (object.date != null)
                        message.date = String(object.date);
                    if (object.content != null)
                        message.content = String(object.content);
                    return message;
                };

                /**
                 * Creates a plain object from a GetNewsSummaryResponse message. Also converts values to other types if specified.
                 * @function toObject
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @static
                 * @param {tldr.news.v1.GetNewsSummaryResponse} message GetNewsSummaryResponse
                 * @param {$protobuf.IConversionOptions} [options] Conversion options
                 * @returns {Object.<string,*>} Plain object
                 */
                GetNewsSummaryResponse.toObject = function toObject(message, options) {
                    if (!options)
                        options = {};
                    let object = {};
                    if (options.defaults) {
                        object.date = "";
                        object.content = "";
                    }
                    if (message.date != null && message.hasOwnProperty("date"))
                        object.date = message.date;
                    if (message.content != null && message.hasOwnProperty("content"))
                        object.content = message.content;
                    return object;
                };

                /**
                 * Converts this GetNewsSummaryResponse to JSON.
                 * @function toJSON
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @instance
                 * @returns {Object.<string,*>} JSON object
                 */
                GetNewsSummaryResponse.prototype.toJSON = function toJSON() {
                    return this.constructor.toObject(this, $protobuf.util.toJSONOptions);
                };

                /**
                 * Gets the default type url for GetNewsSummaryResponse
                 * @function getTypeUrl
                 * @memberof tldr.news.v1.GetNewsSummaryResponse
                 * @static
                 * @param {string} [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns {string} The default type url
                 */
                GetNewsSummaryResponse.getTypeUrl = function getTypeUrl(typeUrlPrefix) {
                    if (typeUrlPrefix === undefined) {
                        typeUrlPrefix = "type.googleapis.com";
                    }
                    return typeUrlPrefix + "/tldr.news.v1.GetNewsSummaryResponse";
                };

                return GetNewsSummaryResponse;
            })();

            return v1;
        })();

        return news;
    })();

    return tldr;
})();

export { $root as default };
