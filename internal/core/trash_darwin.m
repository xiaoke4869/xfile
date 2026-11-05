#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>
char *xfile_trash(const char *path) {
 @autoreleasepool {
  NSError *error = nil;
  NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
  if ([[NSFileManager defaultManager] trashItemAtURL:url resultingItemURL:nil error:&error]) return NULL;
  return strdup(error ? error.localizedDescription.UTF8String : "无法移入废纸篓");
 }
}
