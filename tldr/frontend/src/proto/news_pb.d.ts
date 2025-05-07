import * as $protobuf from "protobufjs";
import Long = require("long");
/** Namespace tldr. */
export namespace tldr {

    /** Namespace news. */
    namespace news {

        /** Namespace v1. */
        namespace v1 {

            /** Properties of a NewsSummary. */
            interface INewsSummary {

                /** NewsSummary date */
                date?: (string|null);
            }

            /** Represents a NewsSummary. */
            class NewsSummary implements INewsSummary {

                /**
                 * Constructs a new NewsSummary.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: tldr.news.v1.INewsSummary);

                /** NewsSummary date. */
                public date: string;

                /**
                 * Creates a new NewsSummary instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns NewsSummary instance
                 */
                public static create(properties?: tldr.news.v1.INewsSummary): tldr.news.v1.NewsSummary;

                /**
                 * Encodes the specified NewsSummary message. Does not implicitly {@link tldr.news.v1.NewsSummary.verify|verify} messages.
                 * @param message NewsSummary message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: tldr.news.v1.INewsSummary, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified NewsSummary message, length delimited. Does not implicitly {@link tldr.news.v1.NewsSummary.verify|verify} messages.
                 * @param message NewsSummary message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: tldr.news.v1.INewsSummary, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a NewsSummary message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns NewsSummary
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): tldr.news.v1.NewsSummary;

                /**
                 * Decodes a NewsSummary message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns NewsSummary
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): tldr.news.v1.NewsSummary;

                /**
                 * Verifies a NewsSummary message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a NewsSummary message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns NewsSummary
                 */
                public static fromObject(object: { [k: string]: any }): tldr.news.v1.NewsSummary;

                /**
                 * Creates a plain object from a NewsSummary message. Also converts values to other types if specified.
                 * @param message NewsSummary
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: tldr.news.v1.NewsSummary, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this NewsSummary to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for NewsSummary
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a ListNewsSummariesResponse. */
            interface IListNewsSummariesResponse {

                /** ListNewsSummariesResponse summaries */
                summaries?: (tldr.news.v1.INewsSummary[]|null);
            }

            /** Represents a ListNewsSummariesResponse. */
            class ListNewsSummariesResponse implements IListNewsSummariesResponse {

                /**
                 * Constructs a new ListNewsSummariesResponse.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: tldr.news.v1.IListNewsSummariesResponse);

                /** ListNewsSummariesResponse summaries. */
                public summaries: tldr.news.v1.INewsSummary[];

                /**
                 * Creates a new ListNewsSummariesResponse instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns ListNewsSummariesResponse instance
                 */
                public static create(properties?: tldr.news.v1.IListNewsSummariesResponse): tldr.news.v1.ListNewsSummariesResponse;

                /**
                 * Encodes the specified ListNewsSummariesResponse message. Does not implicitly {@link tldr.news.v1.ListNewsSummariesResponse.verify|verify} messages.
                 * @param message ListNewsSummariesResponse message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: tldr.news.v1.IListNewsSummariesResponse, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified ListNewsSummariesResponse message, length delimited. Does not implicitly {@link tldr.news.v1.ListNewsSummariesResponse.verify|verify} messages.
                 * @param message ListNewsSummariesResponse message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: tldr.news.v1.IListNewsSummariesResponse, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a ListNewsSummariesResponse message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns ListNewsSummariesResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): tldr.news.v1.ListNewsSummariesResponse;

                /**
                 * Decodes a ListNewsSummariesResponse message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns ListNewsSummariesResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): tldr.news.v1.ListNewsSummariesResponse;

                /**
                 * Verifies a ListNewsSummariesResponse message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a ListNewsSummariesResponse message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns ListNewsSummariesResponse
                 */
                public static fromObject(object: { [k: string]: any }): tldr.news.v1.ListNewsSummariesResponse;

                /**
                 * Creates a plain object from a ListNewsSummariesResponse message. Also converts values to other types if specified.
                 * @param message ListNewsSummariesResponse
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: tldr.news.v1.ListNewsSummariesResponse, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this ListNewsSummariesResponse to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for ListNewsSummariesResponse
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }

            /** Properties of a GetNewsSummaryResponse. */
            interface IGetNewsSummaryResponse {

                /** GetNewsSummaryResponse date */
                date?: (string|null);

                /** GetNewsSummaryResponse content */
                content?: (string|null);
            }

            /** Represents a GetNewsSummaryResponse. */
            class GetNewsSummaryResponse implements IGetNewsSummaryResponse {

                /**
                 * Constructs a new GetNewsSummaryResponse.
                 * @param [properties] Properties to set
                 */
                constructor(properties?: tldr.news.v1.IGetNewsSummaryResponse);

                /** GetNewsSummaryResponse date. */
                public date: string;

                /** GetNewsSummaryResponse content. */
                public content: string;

                /**
                 * Creates a new GetNewsSummaryResponse instance using the specified properties.
                 * @param [properties] Properties to set
                 * @returns GetNewsSummaryResponse instance
                 */
                public static create(properties?: tldr.news.v1.IGetNewsSummaryResponse): tldr.news.v1.GetNewsSummaryResponse;

                /**
                 * Encodes the specified GetNewsSummaryResponse message. Does not implicitly {@link tldr.news.v1.GetNewsSummaryResponse.verify|verify} messages.
                 * @param message GetNewsSummaryResponse message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encode(message: tldr.news.v1.IGetNewsSummaryResponse, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Encodes the specified GetNewsSummaryResponse message, length delimited. Does not implicitly {@link tldr.news.v1.GetNewsSummaryResponse.verify|verify} messages.
                 * @param message GetNewsSummaryResponse message or plain object to encode
                 * @param [writer] Writer to encode to
                 * @returns Writer
                 */
                public static encodeDelimited(message: tldr.news.v1.IGetNewsSummaryResponse, writer?: $protobuf.Writer): $protobuf.Writer;

                /**
                 * Decodes a GetNewsSummaryResponse message from the specified reader or buffer.
                 * @param reader Reader or buffer to decode from
                 * @param [length] Message length if known beforehand
                 * @returns GetNewsSummaryResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decode(reader: ($protobuf.Reader|Uint8Array), length?: number): tldr.news.v1.GetNewsSummaryResponse;

                /**
                 * Decodes a GetNewsSummaryResponse message from the specified reader or buffer, length delimited.
                 * @param reader Reader or buffer to decode from
                 * @returns GetNewsSummaryResponse
                 * @throws {Error} If the payload is not a reader or valid buffer
                 * @throws {$protobuf.util.ProtocolError} If required fields are missing
                 */
                public static decodeDelimited(reader: ($protobuf.Reader|Uint8Array)): tldr.news.v1.GetNewsSummaryResponse;

                /**
                 * Verifies a GetNewsSummaryResponse message.
                 * @param message Plain object to verify
                 * @returns `null` if valid, otherwise the reason why it is not
                 */
                public static verify(message: { [k: string]: any }): (string|null);

                /**
                 * Creates a GetNewsSummaryResponse message from a plain object. Also converts values to their respective internal types.
                 * @param object Plain object
                 * @returns GetNewsSummaryResponse
                 */
                public static fromObject(object: { [k: string]: any }): tldr.news.v1.GetNewsSummaryResponse;

                /**
                 * Creates a plain object from a GetNewsSummaryResponse message. Also converts values to other types if specified.
                 * @param message GetNewsSummaryResponse
                 * @param [options] Conversion options
                 * @returns Plain object
                 */
                public static toObject(message: tldr.news.v1.GetNewsSummaryResponse, options?: $protobuf.IConversionOptions): { [k: string]: any };

                /**
                 * Converts this GetNewsSummaryResponse to JSON.
                 * @returns JSON object
                 */
                public toJSON(): { [k: string]: any };

                /**
                 * Gets the default type url for GetNewsSummaryResponse
                 * @param [typeUrlPrefix] your custom typeUrlPrefix(default "type.googleapis.com")
                 * @returns The default type url
                 */
                public static getTypeUrl(typeUrlPrefix?: string): string;
            }
        }
    }
}
