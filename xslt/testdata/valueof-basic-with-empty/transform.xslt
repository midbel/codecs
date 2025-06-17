<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<link>
			<xsl:variable name="link" select="/root/item"/>
			<div>
				<a href="{$link}.xml" class="btn btn-default">
					<div>
						<i class="icon icon-see"></i>
						<span>
							<xsl:value-of select="$link"/>
						</span>
					</div>
				</a>
			</div>
		</link>
	</xsl:template>
</xsl:stylesheet>