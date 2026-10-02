#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>

char *ar_plist_xml(const void *json, size_t length, char **message) {
    @autoreleasepool {
        NSError *error = nil;
        NSData *input = [NSData dataWithBytes:json length:length];
        id values = [NSJSONSerialization JSONObjectWithData:input options:0 error:&error];
        if (!values) {
            *message = strdup(error.localizedDescription.UTF8String);
            return NULL;
        }
        if (![NSPropertyListSerialization propertyList:values isValidForFormat:NSPropertyListXMLFormat_v1_0]) {
            *message = strdup("values cannot be represented as an Apple property list");
            return NULL;
        }
        NSData *output = [NSPropertyListSerialization dataWithPropertyList:values
            format:NSPropertyListXMLFormat_v1_0 options:0 error:&error];
        if (!output) {
            *message = strdup(error.localizedDescription.UTF8String);
            return NULL;
        }
        char *result = malloc(output.length + 1);
        if (!result) return NULL;
        memcpy(result, output.bytes, output.length);
        result[output.length] = '\0';
        return result;
    }
}
